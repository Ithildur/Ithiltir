package config

import (
	"fmt"

	kitauth "github.com/Ithildur/EiluneKit/auth"
)

const AdminPasswordMinLength = 8

func ValidateAdminPassword(password string) error {
	if err := kitauth.ValidateStaticPassword(password); err != nil {
		return err
	}
	if len(password) < AdminPasswordMinLength {
		return fmt.Errorf("password must contain at least %d characters", AdminPasswordMinLength)
	}
	return nil
}
