package store

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"streammon/internal/models"
)

func seedAliasUsers(t *testing.T, s *Store, names ...string) map[string]int64 {
	t.Helper()
	ids := make(map[string]int64, len(names))
	for _, n := range names {
		u, err := s.GetOrCreateUser(n)
		if err != nil {
			t.Fatalf("GetOrCreateUser(%q): %v", n, err)
		}
		ids[n] = u.ID
	}
	return ids
}

func countHistoryFor(t *testing.T, s *Store, name string) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM watch_history WHERE user_name = ?`, name).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func insertAliasHistory(t *testing.T, s *Store, serverID int64, name string) {
	t.Helper()
	if _, err := s.db.Exec(`INSERT INTO watch_history (server_id, user_name, title, media_type, started_at, stopped_at)
		VALUES (?, ?, 'Movie', 'movie', '2024-01-01T00:00:00Z', '2024-01-01T02:00:00Z')`, serverID, name); err != nil {
		t.Fatal(err)
	}
}

func TestMergeUsers_RecordsAlias(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob")

	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	aliases, err := s.ListUserAliases("alice")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(aliases, []string{"bob"}) {
		t.Errorf("aliases = %v, want [bob]", aliases)
	}
}

func TestMergeUsers_CarriesAliasesToNewKeepUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob", "carol")

	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}
	if _, err := s.MergeUsers(ids["carol"], ids["alice"]); err != nil {
		t.Fatal(err)
	}

	got, err := s.UserAliases()
	if err != nil {
		t.Fatal(err)
	}
	want := UserAliasMap{"alice": "carol", "bob": "carol"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UserAliases = %v, want %v", got, want)
	}
}

func TestDeleteUser_DropsAliases(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteUser(ids["alice"]); err != nil {
		t.Fatal(err)
	}

	got, err := s.UserAliases()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("UserAliases = %v, want empty", got)
	}
}

func TestLinkUserAlias_MergesExistingUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	seedAliasUsers(t, s, "alice", "bob")
	insertAliasHistory(t, s, serverID, "bob")

	result, err := s.LinkUserAlias(context.Background(), "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if result.WatchHistoryMoved != 1 {
		t.Errorf("WatchHistoryMoved = %d, want 1", result.WatchHistoryMoved)
	}
	if _, err := s.GetUser("bob"); err == nil {
		t.Error("bob should be deleted")
	}
	if n := countHistoryFor(t, s, "alice"); n != 1 {
		t.Errorf("alice history = %d, want 1", n)
	}
	aliases, _ := s.ListUserAliases("alice")
	if !reflect.DeepEqual(aliases, []string{"bob"}) {
		t.Errorf("aliases = %v, want [bob]", aliases)
	}
}

func TestLinkUserAlias_NameWithOnlyHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	seedAliasUsers(t, s, "alice")
	insertAliasHistory(t, s, serverID, "bob")

	result, err := s.LinkUserAlias(context.Background(), "alice", "bob")
	if err != nil {
		t.Fatal(err)
	}
	if result.WatchHistoryMoved != 1 {
		t.Errorf("WatchHistoryMoved = %d, want 1", result.WatchHistoryMoved)
	}
	if _, err := s.GetUser("bob"); err == nil {
		t.Error("linking must not leave a bob user row behind")
	}
	aliases, _ := s.ListUserAliases("alice")
	if !reflect.DeepEqual(aliases, []string{"bob"}) {
		t.Errorf("aliases = %v, want [bob]", aliases)
	}
}

func TestLinkUserAlias_Rejects(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	seedAliasUsers(t, s, "alice")

	if _, err := s.LinkUserAlias(context.Background(), "alice", "alice"); !errors.Is(err, ErrInvalidAlias) {
		t.Errorf("self alias: err = %v, want ErrInvalidAlias", err)
	}
	if _, err := s.LinkUserAlias(context.Background(), "alice", "  "); !errors.Is(err, ErrInvalidAlias) {
		t.Errorf("blank alias: err = %v, want ErrInvalidAlias", err)
	}
	if _, err := s.LinkUserAlias(context.Background(), "nobody", "bob"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("unknown user: err = %v, want ErrNotFound", err)
	}
}

func TestUnlinkUserAlias(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob", "carol")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	if err := s.UnlinkUserAlias("carol", "bob"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("other user's alias: err = %v, want ErrNotFound", err)
	}
	if err := s.UnlinkUserAlias("alice", "bob"); err != nil {
		t.Fatal(err)
	}
	aliases, _ := s.ListUserAliases("alice")
	if len(aliases) != 0 {
		t.Errorf("aliases = %v, want empty", aliases)
	}
	if err := s.UnlinkUserAlias("alice", "bob"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("second unlink: err = %v, want ErrNotFound", err)
	}
}

func TestListUserAliases_EmptyIsNonNil(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	seedAliasUsers(t, s, "alice")

	aliases, err := s.ListUserAliases("alice")
	if err != nil {
		t.Fatal(err)
	}
	if aliases == nil {
		t.Error("aliases should be an empty slice, not nil")
	}
}

func TestInsertHistoryBatch_ResolvesAliases(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	ids := seedAliasUsers(t, s, "alice", "bob")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entries := []*models.WatchHistoryEntry{{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Movie", StartedAt: start, StoppedAt: start.Add(time.Hour),
	}}
	if _, _, _, err := s.InsertHistoryBatch(context.Background(), entries); err != nil {
		t.Fatal(err)
	}

	if n := countHistoryFor(t, s, "alice"); n != 1 {
		t.Errorf("alice history = %d, want 1", n)
	}
	if n := countHistoryFor(t, s, "bob"); n != 0 {
		t.Errorf("bob history = %d, want 0", n)
	}
}

func TestSyncUsersFromServer_SkipsAliases(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	users := []models.MediaUser{{Name: "bob", ThumbURL: "user/abc"}}
	if _, err := s.SyncUsersFromServer(3, "jellyfin", users); err != nil {
		t.Fatal(err)
	}

	if _, err := s.GetUser("bob"); err == nil {
		t.Error("sync must not recreate a merged-away user")
	}
}

func TestInsertHistory_ResolvesAliases(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	ids := seedAliasUsers(t, s, "alice", "bob")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}

	start := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
	entry := &models.WatchHistoryEntry{
		ServerID: serverID, UserName: "bob", MediaType: models.MediaTypeMovie,
		Title: "Movie", StartedAt: start, StoppedAt: start.Add(time.Hour),
	}
	if err := s.InsertHistory(entry); err != nil {
		t.Fatal(err)
	}

	if n := countHistoryFor(t, s, "alice"); n != 1 {
		t.Errorf("alice history = %d, want 1", n)
	}
}

func TestLinkUserAlias_AliasOwnedByAnotherUser(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ctx := context.Background()
	seedAliasUsers(t, s, "alice", "bob", "carol")
	if _, err := s.LinkUserAlias(ctx, "alice", "bob"); err != nil {
		t.Fatal(err)
	}

	if _, err := s.LinkUserAlias(ctx, "carol", "bob"); !errors.Is(err, ErrAliasTaken) {
		t.Errorf("err = %v, want ErrAliasTaken", err)
	}
	if _, err := s.LinkUserAlias(ctx, "alice", "bob"); err != nil {
		t.Errorf("relinking to the same user should be a no-op, got %v", err)
	}
	if _, err := s.GetUser("bob"); err == nil {
		t.Error("a rejected or repeated link must not recreate bob")
	}
	got, _ := s.UserAliases()
	if got["bob"] != "alice" {
		t.Errorf("bob maps to %q, want alice", got["bob"])
	}
}

func TestLinkUserAlias_KeepUserWithOnlyHistory(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	serverID := seedServer(t, s)
	insertAliasHistory(t, s, serverID, "alice")
	insertAliasHistory(t, s, serverID, "bob")

	if _, err := s.LinkUserAlias(context.Background(), "alice", "bob"); err != nil {
		t.Fatal(err)
	}
	if n := countHistoryFor(t, s, "alice"); n != 2 {
		t.Errorf("alice history = %d, want 2", n)
	}
	if _, err := s.LinkUserAlias(context.Background(), "nobody", "bob2"); !errors.Is(err, models.ErrNotFound) {
		t.Errorf("unknown user without history: err = %v, want ErrNotFound", err)
	}
}

func TestMergeUsers_DropsAliasMatchingKeepName(t *testing.T) {
	s := newTestStoreWithMigrations(t)
	ids := seedAliasUsers(t, s, "alice", "bob")
	if _, err := s.MergeUsers(ids["alice"], ids["bob"]); err != nil {
		t.Fatal(err)
	}
	recreated := seedAliasUsers(t, s, "bob", "carol")

	if _, err := s.MergeUsers(recreated["bob"], recreated["carol"]); err != nil {
		t.Fatal(err)
	}

	got, _ := s.UserAliases()
	want := UserAliasMap{"carol": "bob"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("UserAliases = %v, want %v", got, want)
	}
}
