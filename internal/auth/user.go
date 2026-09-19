package auth

import (
	"fmt"
	"strings"
)

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

type Name struct {
	Last   string
	First  string
	Middle string
}

func (n Name) Full() string {
	return strings.Join(strings.Fields(n.Last+" "+n.First+" "+n.Middle), " ")
}

type User struct {
	ID       int64
	Login    string
	Name     Name
	FullName string
	Role     Role
	Active   bool
}
