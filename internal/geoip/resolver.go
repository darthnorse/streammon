package geoip

import (
	"log"
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"

	"streammon/internal/models"
)

type Resolver struct {
	mu    sync.RWMutex
	db    *maxminddb.Reader
	asnDB *maxminddb.Reader
}

type mmdbRecord struct {
	City struct {
		Names map[string]string `maxminddb:"names"`
	} `maxminddb:"city"`
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Location struct {
		Latitude  float64 `maxminddb:"latitude"`
		Longitude float64 `maxminddb:"longitude"`
	} `maxminddb:"location"`
}

type asnRecord struct {
	AutonomousSystemOrganization string `maxminddb:"autonomous_system_organization"`
}

func NewResolver(dbPath string) *Resolver {
	if dbPath == "" {
		return &Resolver{}
	}
	db, err := maxminddb.Open(dbPath)
	if err != nil {
		log.Printf("geoip: failed to open %s: %v", dbPath, err)
		return &Resolver{}
	}
	return &Resolver{db: db}
}

func (r *Resolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.db != nil {
		r.db.Close()
	}
	if r.asnDB != nil {
		r.asnDB.Close()
	}
	return nil
}

func (r *Resolver) Lookup(ip net.IP) *models.GeoResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if ip == nil || r.db == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() {
		return nil
	}
	var record mmdbRecord
	err := r.db.Lookup(ip, &record)
	if err != nil {
		return nil
	}
	city := record.City.Names["en"]
	result := &models.GeoResult{
		IP:      ip.String(),
		Lat:     record.Location.Latitude,
		Lng:     record.Location.Longitude,
		City:    city,
		Country: record.Country.ISOCode,
	}

	if r.asnDB != nil {
		var asn asnRecord
		if err := r.asnDB.Lookup(ip, &asn); err == nil {
			result.ISP = asn.AutonomousSystemOrganization
		}
	}

	return result
}

func (r *Resolver) Reload(dbPath string) error {
	newDB, err := maxminddb.Open(dbPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	old := r.db
	r.db = newDB
	r.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}

func (r *Resolver) ReloadASN(dbPath string) error {
	newDB, err := maxminddb.Open(dbPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	old := r.asnDB
	r.asnDB = newDB
	r.mu.Unlock()
	if old != nil {
		old.Close()
	}
	return nil
}
