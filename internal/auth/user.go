package auth

import "fmt"

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleTeacher Role = "teacher"
	RoleStudent Role = "student"
	RoleParent  Role = "parent"
)

func ParseRole(s string) (Role, error) {
	switch role := Role(s); role {
	case RoleAdmin, RoleTeacher, RoleStudent, RoleParent:
		return role, nil
	default:
		return "", fmt.Errorf("unknown role %q", s)
	}
}

type User struct {
	ID       int64
	Login    string
	FullName string
	Role     Role
	Active   bool
}
