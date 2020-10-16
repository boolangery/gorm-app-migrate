package apps

import (
	"errors"
	"gorm.io/gorm"
)

// check is its a real error
func IsDBError(result *gorm.DB) bool {
	return result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound)
}
