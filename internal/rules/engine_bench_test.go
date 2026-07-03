package rules

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"streammon/internal/models"
	"streammon/internal/store"
)

// benchGeoResolver returns a deterministic, IP-derived location so geo-dependent
// evaluators exercise their full code path (including their second history/geo
// lookups) without needing a real GeoIP database.
type benchGeoResolver struct{}

func (benchGeoResolver) Lookup(ctx context.Context, ip string) (*models.GeoResult, error) {
	if ip == "" {
		return nil, nil
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(ip))
	v := h.Sum32()
	lat := (float64(v%18000) / 100.0) - 90.0
	lng := (float64((v/18000)%36000) / 100.0) - 180.0
	return &models.GeoResult{IP: ip, Country: "US", City: "City-" + ip, Lat: lat, Lng: lng}, nil
}

func copyFileBench(tb testing.TB, src, dst string) {
	tb.Helper()
	in, err := os.Open(src)
	if err != nil {
		tb.Fatalf("open bench db %q: %v", src, err)
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		tb.Fatalf("create temp db: %v", err)
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		tb.Fatalf("copy bench db: %v", err)
	}
}

func createBenchRules(tb testing.TB, s *store.Store) {
	tb.Helper()
	type ruleSpec struct {
		name string
		typ  models.RuleType
		cfg  interface{}
	}
	specs := []ruleSpec{
		{"concurrent", models.RuleTypeConcurrentStreams, models.ConcurrentStreamsConfig{MaxStreams: 2}},
		{"geo", models.RuleTypeGeoRestriction, models.GeoRestrictionConfig{AllowedCountries: []string{"US"}}},
		{"simultaneous", models.RuleTypeSimultaneousLocs, models.SimultaneousLocsConfig{MinDistanceKm: 50}},
		{"travel", models.RuleTypeImpossibleTravel, models.ImpossibleTravelConfig{MaxSpeedKmH: 800, MinDistanceKm: 100, TimeWindowHours: 24}},
		{"devvel", models.RuleTypeDeviceVelocity, models.DeviceVelocityConfig{MaxDevicesPerHour: 3, TimeWindowHours: 1}},
		{"newdev", models.RuleTypeNewDevice, models.NewDeviceConfig{NotifyOnNew: true}},
		{"newloc", models.RuleTypeNewLocation, models.NewLocationConfig{NotifyOnNew: true, MinDistanceKm: 50}},
		{"ispvel", models.RuleTypeISPVelocity, models.ISPVelocityConfig{MaxISPs: 3, TimeWindowHours: 168}},
	}
	for _, sp := range specs {
		cfg, _ := json.Marshal(sp.cfg)
		rule := &models.Rule{Name: sp.name, Type: sp.typ, Enabled: true, Config: cfg}
		if err := s.CreateRule(rule); err != nil {
			tb.Fatalf("create rule %s: %v", sp.name, err)
		}
	}
}

// buildBenchStreams reads the busiest real users from the benchmark DB and turns
// each into an active stream started "now" so the history-querying evaluators run
// against realistic per-user data.
func buildBenchStreams(tb testing.TB, dbPath string, n int) []models.ActiveStream {
	tb.Helper()
	raw, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		tb.Fatalf("open raw db: %v", err)
	}
	defer raw.Close()

	rows, err := raw.Query(`
		SELECT user_name, ip_address, player, platform
		FROM watch_history w1
		WHERE started_at = (SELECT MAX(started_at) FROM watch_history w2 WHERE w2.user_name = w1.user_name)
		GROUP BY user_name
		ORDER BY (SELECT COUNT(*) FROM watch_history w3 WHERE w3.user_name = w1.user_name) DESC
		LIMIT 40`)
	if err != nil {
		tb.Fatalf("query users: %v", err)
	}
	defer rows.Close()

	type u struct{ name, ip, player, platform string }
	var users []u
	for rows.Next() {
		var x u
		if err := rows.Scan(&x.name, &x.ip, &x.player, &x.platform); err != nil {
			tb.Fatalf("scan: %v", err)
		}
		users = append(users, x)
	}
	if len(users) == 0 {
		tb.Fatalf("no users in bench db")
	}

	now := time.Now().UTC()
	streams := make([]models.ActiveStream, 0, n)
	for i := 0; i < n; i++ {
		x := users[i%len(users)]
		streams = append(streams, models.ActiveStream{
			SessionID: fmt.Sprintf("bench-%d", i),
			ServerID:  1,
			UserName:  x.name,
			IPAddress: x.ip,
			Player:    x.player,
			Platform:  x.platform,
			State:     models.SessionStatePlaying,
			StartedAt: now,
		})
	}
	return streams
}

// buildBenchStreamsSameUser builds n streams that all belong to the busiest
// user (the realistic "concurrent streams" tick), so per-user reads that the
// batched path caches (households) are exercised n times in the per-session path.
func buildBenchStreamsSameUser(tb testing.TB, dbPath string, n int) []models.ActiveStream {
	tb.Helper()
	base := buildBenchStreams(tb, dbPath, 1)
	u := base[0]
	streams := make([]models.ActiveStream, 0, n)
	for i := 0; i < n; i++ {
		s := u
		s.SessionID = fmt.Sprintf("same-%d", i)
		streams = append(streams, s)
	}
	return streams
}

func setupBenchEngine(b *testing.B) (*Engine, []models.ActiveStream, []models.ActiveStream) {
	src := os.Getenv("STREAMMON_BENCH_DB")
	if src == "" {
		b.Skip("set STREAMMON_BENCH_DB to a copy of a real streammon.db to run this benchmark")
	}
	dst := filepath.Join(b.TempDir(), "bench.db")
	copyFileBench(b, src, dst)

	s, err := store.New(dst)
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		b.Fatalf("migrate: %v", err)
	}
	b.Cleanup(func() { s.Close() })

	createBenchRules(b, s)

	e := NewEngine(s, benchGeoResolver{}, DefaultEngineConfig())
	if err := e.RefreshRules(); err != nil {
		b.Fatalf("refresh rules: %v", err)
	}

	streams5 := buildBenchStreams(b, dst, 5)
	streams20 := buildBenchStreams(b, dst, 20)
	return e, streams5, streams20
}

// BenchmarkRuleEval_PerSession measures the current poll() behavior: one
// EvaluateSession call per active session per tick.
func BenchmarkRuleEval_PerSession(b *testing.B) {
	e, streams5, streams20 := setupBenchEngine(b)
	ctx := context.Background()

	b.Run("N5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := range streams5 {
				e.EvaluateSession(ctx, &streams5[j], streams5)
			}
		}
	})
	b.Run("N20", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := range streams20 {
				e.EvaluateSession(ctx, &streams20[j], streams20)
			}
		}
	})
}

// BenchmarkRuleEval_Batched measures the new per-tick path: a single
// EvaluateSessions call that hoists the per-tick-constant reads.
func BenchmarkRuleEval_Batched(b *testing.B) {
	e, streams5, streams20 := setupBenchEngine(b)
	ctx := context.Background()

	b.Run("N5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e.EvaluateSessions(ctx, streams5)
		}
	})
	b.Run("N20", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e.EvaluateSessions(ctx, streams20)
		}
	})
}

// BenchmarkRuleEval_SameUser compares the two paths for a tick where all
// sessions belong to one user (household reads are cached once in the batched
// path but repeated per session in the old path).
func BenchmarkRuleEval_SameUser(b *testing.B) {
	src := os.Getenv("STREAMMON_BENCH_DB")
	if src == "" {
		b.Skip("set STREAMMON_BENCH_DB to a copy of a real streammon.db to run this benchmark")
	}
	dst := filepath.Join(b.TempDir(), "bench.db")
	copyFileBench(b, src, dst)
	s, err := store.New(dst)
	if err != nil {
		b.Fatalf("open store: %v", err)
	}
	if err := s.Migrate("../../migrations"); err != nil {
		b.Fatalf("migrate: %v", err)
	}
	b.Cleanup(func() { s.Close() })
	createBenchRules(b, s)
	e := NewEngine(s, benchGeoResolver{}, DefaultEngineConfig())
	if err := e.RefreshRules(); err != nil {
		b.Fatalf("refresh rules: %v", err)
	}

	streams := buildBenchStreamsSameUser(b, dst, 10)
	ctx := context.Background()

	b.Run("PerSession-N10", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			for j := range streams {
				e.EvaluateSession(ctx, &streams[j], streams)
			}
		}
	})
	b.Run("Batched-N10", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			e.EvaluateSessions(ctx, streams)
		}
	})
}
