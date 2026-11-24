// Package errcode defines stable business error numbers in Cloud API contracts.
package errcode

const (
	PublishInvalid        = 18001
	PublishNotFound       = 18002
	PublishConflict       = 18003
	PublishOutputOccupied = 18004
	PublishPlatformClosed = 18005
	PublishUnavailable    = 18006

	PublishBatchParentBlocked    = 18007
	PublishBatchShortfallChanged = 18008
	PublishBatchPauseChanged     = 18009
	PublishBatchUnresolved       = 18010
	PublishCapacityWait          = 18011
	PublishAgentUpgradeRequired  = 18012
	PublishProfileUseConflict    = 18013
	PublishAdmissionClosed       = 18014
)
