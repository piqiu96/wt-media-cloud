package executionpreflight

import (
	"database/sql"
	"errors"

	"github.com/wt-media/wt-media-cloud/internal/infra/database"
	"gorm.io/gorm"
)

// LoadCloudFacts 读取准入判断依赖的账号与 Profile 事实。
func LoadCloudFacts(userID int64, profileID, mediaAccountID string) (CloudFacts, error) {
	return queryCloudFacts(database.DB(), userID, profileID, mediaAccountID)
}

func queryCloudFacts(db *gorm.DB, userID int64, profileID, mediaAccountID string) (CloudFacts, error) {
	facts := CloudFacts{}
	var status string
	var mainUserID sql.NullString
	err := db.Raw(`SELECT status, bit_main_user_id FROM users WHERE id = ?`, userID).Row().Scan(&status, &mainUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return facts, nil
	}
	if err != nil {
		return facts, err
	}
	facts.UserEnabled = status == "enabled"
	facts.BitMainUserID = mainUserID.String

	if profileID != "" {
		facts.Profile, err = loadProfileFacts(db, userID, profileID)
		if err != nil {
			return CloudFacts{}, err
		}
	}
	if mediaAccountID != "" {
		facts.MediaAccount, err = loadMediaAccountFacts(db, userID, mediaAccountID)
		if err != nil {
			return CloudFacts{}, err
		}
	}
	return facts, nil
}

func loadProfileFacts(db *gorm.DB, userID int64, profileID string) (ProfileFacts, error) {
	var ownerID int64
	var mainUserID, localStatus string
	err := db.Raw(`SELECT user_id, COALESCE(main_user_id, ''), local_status
		FROM browser_profiles WHERE id = ?`, profileID).Row().Scan(&ownerID, &mainUserID, &localStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return ProfileFacts{}, nil
	}
	if err != nil {
		return ProfileFacts{}, err
	}
	return ProfileFacts{
		Found:               true,
		OwnedByUser:         ownerID == userID,
		AssignedToOtherUser: ownerID != userID,
		MainUserID:          mainUserID,
		LocalStatus:         localStatus,
	}, nil
}

func loadMediaAccountFacts(db *gorm.DB, userID int64, mediaAccountID string) (MediaAccountFacts, error) {
	var ownerID int64
	var loginStatus string
	err := db.Raw(`SELECT user_id, login_status FROM media_accounts WHERE id = ?`, mediaAccountID).Row().Scan(&ownerID, &loginStatus)
	if errors.Is(err, sql.ErrNoRows) {
		return MediaAccountFacts{}, nil
	}
	if err != nil {
		return MediaAccountFacts{}, err
	}
	if ownerID != userID {
		return MediaAccountFacts{}, nil
	}
	return MediaAccountFacts{Found: true, LoginStatus: loginStatus}, nil
}
