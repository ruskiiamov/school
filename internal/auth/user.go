package auth

import "fmt"

type Role string

const (
	RoleAdmin   Role = "admin"
	RoleTeacher Role = "teacher"
	RoleStudent Role = "student"
	RoleParent  Role = "parent"
)

var roleTitles = map[Role]string{
	RoleAdmin:   "Администратор",
	RoleTeacher: "Учитель",
	RoleStudent: "Ученик",
	RoleParent:  "Родитель",
}

func ParseRole(s string) (Role, error) {
	role := Role(s)
	if _, ok := roleTitles[role]; !ok {
		return "", fmt.Errorf("unknown role %q", s)
	}

	return role, nil
}

func (r Role) Title() string {
	if title, ok := roleTitles[r]; ok {
		return title
	}

	return string(r)
}

type User struct {
	ID       int64
	Login    string
	FullName string
	Role     Role
}
