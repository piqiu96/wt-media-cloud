package service

import identityservice "github.com/wt-media/wt-media-cloud/internal/modules/identity/service"

func testCategoryID() *identityservice.CategoryID {
	value := identityservice.CategoryID(1)
	return &value
}
