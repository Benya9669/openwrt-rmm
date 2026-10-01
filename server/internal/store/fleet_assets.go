package store

import (
	"context"
	"errors"
	"strings"
	"time"
)

type FleetAsset struct {
	DeviceID      string `json:"device_id"`
	Hostname      string `json:"hostname"`
	Model         string `json:"model"`
	SerialNumber  string `json:"serial_number"`
	Site          string `json:"site"`
	Responsible   string `json:"responsible"`
	WarrantyUntil string `json:"warranty_until"`
	Notes         string `json:"notes"`
	UpdatedAt     string `json:"updated_at"`
}

func (s *Store) SaveFleetAsset(ctx context.Context, userID string, asset FleetAsset) error {
	for _, value := range []string{asset.Model, asset.SerialNumber, asset.Site, asset.Responsible} {
		if len(value) > 255 {
			return errors.New("equipment field exceeds 255 bytes")
		}
	}
	if len(asset.Notes) > 4096 {
		return errors.New("equipment notes exceed 4096 bytes")
	}
	if asset.WarrantyUntil != "" {
		if _, err := time.Parse("2006-01-02", asset.WarrantyUntil); err != nil {
			return errors.New("warranty date must be YYYY-MM-DD")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return err
	}
	if err := fleetDeviceAccess(ctx, tx, userID, asset.DeviceID, admin); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO fleet_assets (device_id,model,serial_number,site,responsible,warranty_until,notes,updated_at) VALUES (?,?,?,?,?,?,?,?) ON CONFLICT (device_id) DO UPDATE SET model=excluded.model,serial_number=excluded.serial_number,site=excluded.site,responsible=excluded.responsible,warranty_until=excluded.warranty_until,notes=excluded.notes,updated_at=excluded.updated_at", asset.DeviceID, strings.TrimSpace(asset.Model), strings.TrimSpace(asset.SerialNumber), strings.TrimSpace(asset.Site), strings.TrimSpace(asset.Responsible), asset.WarrantyUntil, strings.TrimSpace(asset.Notes), nowText()); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) ListFleetAssets(ctx context.Context, userID, search, site string) ([]FleetAsset, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	admin, err := fleetPrincipal(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, "SELECT d.id,d.hostname,COALESCE(a.model,''),COALESCE(a.serial_number,''),COALESCE(a.site,''),COALESCE(a.responsible,''),COALESCE(a.warranty_until,''),COALESCE(a.notes,''),COALESCE(a.updated_at,'') FROM devices d LEFT JOIN fleet_assets a ON a.device_id=d.id WHERE (d.owner_user_id=? OR ?=1) ORDER BY d.hostname,d.id", userID, admin)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets := []FleetAsset{}
	search = strings.ToLower(strings.TrimSpace(search))
	for rows.Next() {
		var asset FleetAsset
		if err := rows.Scan(&asset.DeviceID, &asset.Hostname, &asset.Model, &asset.SerialNumber, &asset.Site, &asset.Responsible, &asset.WarrantyUntil, &asset.Notes, &asset.UpdatedAt); err != nil {
			return nil, err
		}
		if site != "" && asset.Site != site {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(strings.Join([]string{asset.Hostname, asset.Model, asset.SerialNumber, asset.Site, asset.Responsible, asset.Notes}, "\n")), search) {
			continue
		}
		assets = append(assets, asset)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	visible := assets[:0]
	for _, asset := range assets {
		if err = permissionAllowed(ctx, tx, userID, asset.DeviceID, "view", admin); errors.Is(err, ErrFleetAccess) {
			continue
		} else if err != nil {
			return nil, err
		}
		visible = append(visible, asset)
	}
	return visible, nil
}
