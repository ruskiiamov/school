package view

type Icon string

const (
	IconMenu      Icon = "menu"
	IconLogout    Icon = "logout"
	IconUser      Icon = "user"
	IconBook      Icon = "book"
	IconDashboard Icon = "dashboard"
	IconClasses   Icon = "classes"
	IconSubjects  Icon = "subjects"
	IconTeachers  Icon = "teachers"
	IconStudents  Icon = "students"
	IconSchedule  Icon = "schedule"
	IconMarks     Icon = "marks"
	IconClock     Icon = "clock"
	IconMegaphone Icon = "megaphone"
	IconAlert     Icon = "alert"
)

type User struct {
	FullName string
	Role     string
}

type NavItem struct {
	Title    string
	Href     string
	Icon     Icon
	Active   bool
	Disabled bool
}

type Shell struct {
	Title      string
	SchoolName string
	User       User
	Nav        []NavItem
}

type Stat struct {
	Label string
	Value string
	Icon  Icon
}

type LoginPage struct {
	SchoolName string
	Login      string
	Error      string
}

type HomePage struct {
	Shell Shell
	Stats []Stat
	Today string
}
