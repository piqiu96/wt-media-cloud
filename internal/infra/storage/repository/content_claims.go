package repository

import (
	"context"

	"gorm.io/gorm"
)

// ContentClaims reserves immutable content identity across all Cloud processes.
// Claims outlive objects: deletion never permits a different payload at an old
// formal filename. Replacements must allocate a new asset identity.
type ContentClaims struct{ DB *gorm.DB }

func (c ContentClaims) Claim(ctx context.Context, locationHash, digest string, size int64) (bool, error) {
	db := c.DB.WithContext(ctx)
	if err := db.Exec(`INSERT INTO storage_content_claims (location_hash,sha256,byte_size) VALUES (?,?,?) ON DUPLICATE KEY UPDATE location_hash=location_hash`, locationHash, digest, size).Error; err != nil {
		return false, err
	}
	var storedDigest string
	var storedSize int64
	if err := db.Raw(`SELECT sha256,byte_size FROM storage_content_claims WHERE location_hash=?`, locationHash).Row().Scan(&storedDigest, &storedSize); err != nil {
		return false, err
	}
	return storedDigest == digest && storedSize == size, nil
}
