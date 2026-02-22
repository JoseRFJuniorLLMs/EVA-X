// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

package nietzsche

import (
	"context"
	"time"

	"eva/internal/brainstem/logger"

	nietzsche "nietzsche-sdk"
)

// BackupService provides automated and manual backup/restore for NietzscheDB.
type BackupService struct {
	client   *Client
	interval time.Duration
}

// NewBackupService creates a backup service with the given interval (e.g. 24h for daily).
func NewBackupService(client *Client, interval time.Duration) *BackupService {
	return &BackupService{client: client, interval: interval}
}

// Start begins the automated backup loop. Blocks until ctx is cancelled.
// Call in a goroutine: go backupService.Start(ctx)
func (bs *BackupService) Start(ctx context.Context) {
	log := logger.Nietzsche()
	log.Info().Dur("interval", bs.interval).Msg("[Backup] Automated backup service started")

	ticker := time.NewTicker(bs.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("[Backup] Automated backup service stopped")
			return
		case t := <-ticker.C:
			label := "auto-" + t.Format("20060102-150405")
			info, err := bs.client.CreateBackup(ctx, label)
			if err != nil {
				log.Error().Err(err).Str("label", label).Msg("[Backup] Automated backup failed")
				continue
			}
			log.Info().
				Str("label", info.Label).
				Str("path", info.Path).
				Uint64("size_bytes", info.SizeBytes).
				Msg("[Backup] Automated backup completed")
		}
	}
}

// ManualBackup creates a one-off backup with a custom label.
func (bs *BackupService) ManualBackup(ctx context.Context, label string) (nietzsche.BackupInfo, error) {
	log := logger.Nietzsche()

	info, err := bs.client.CreateBackup(ctx, label)
	if err != nil {
		log.Error().Err(err).Str("label", label).Msg("[Backup] Manual backup failed")
		return nietzsche.BackupInfo{}, err
	}

	log.Info().
		Str("label", info.Label).
		Str("path", info.Path).
		Uint64("size_bytes", info.SizeBytes).
		Msg("[Backup] Manual backup completed")
	return info, nil
}

// ListBackups returns all available backups.
func (bs *BackupService) ListBackups(ctx context.Context) ([]nietzsche.BackupInfo, error) {
	return bs.client.ListBackups(ctx)
}

// Restore restores NietzscheDB from a backup path.
func (bs *BackupService) Restore(ctx context.Context, backupPath, targetPath string) error {
	log := logger.Nietzsche()

	if err := bs.client.RestoreBackup(ctx, backupPath, targetPath); err != nil {
		log.Error().Err(err).Str("backup", backupPath).Msg("[Backup] Restore failed")
		return err
	}

	log.Info().Str("backup", backupPath).Str("target", targetPath).Msg("[Backup] Restore completed")
	return nil
}
