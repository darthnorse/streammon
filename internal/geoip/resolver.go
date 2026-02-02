package geoip

import (
	"log"
	"net"

	"github.com/oschwald/maxminddb-golang"

	"streammon/internal/models"
)

type Resolver struct {
	db *maxminddb.Reader
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
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

func (r *Resolver) Lookup(ip net.IP) *models.GeoResult {
	if ip == nil || r.db == nil || ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() {
		return nil
	}
	var record mmdbRecord
	err := r.db.Lookup(ip, &record)
	if err != nil {
		return nil
	}
	city := record.City.Names["en"]
	return &models.GeoResult{
		IP:      ip.String(),
		Lat:     record.Location.Latitude,
		Lng:     record.Location.Longitude,
		City:    city,
		Country: record.Country.ISOCode,
	}
}
