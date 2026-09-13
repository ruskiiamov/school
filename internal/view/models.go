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
	IconParents   Icon = "parents"
	IconWorkTypes Icon = "work-types"
	IconMarks     Icon = "marks"
	IconDiary     Icon = "diary"
	IconClock     Icon = "clock"
	IconMegaphone Icon = "megaphone"
	IconAlert     Icon = "alert"
	IconPlus      Icon = "plus"
	IconUp        Icon = "up"
	IconDown      Icon = "down"
)

type User struct {
	FullName string
	Role     string
}

type NavItem struct {
	Title  string
	Href   string
	Icon   Icon
	Active bool
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

type StubPage struct {
	Shell   Shell
	Heading string
}

type SubjectRow struct {
	ID      int64
	Name    string
	Active  bool
	Editing bool
	Error   string
}

type SubjectsPage struct {
	Shell        Shell
	Subjects     []SubjectRow
	ShowInactive bool
	NewName      string
	NewError     string
}

type WorkTypeRow struct {
	ID      int64
	Name    string
	Active  bool
	Editing bool
	CanUp   bool
	CanDown bool
	Error   string
}

type WorkTypesPage struct {
	Shell        Shell
	WorkTypes    []WorkTypeRow
	ShowInactive bool
	NewName      string
	NewError     string
}

type YearOption struct {
	Year   int
	Name   string
	Href   string
	Active bool
}

type ClassRow struct {
	ID      int64
	Name    string
	Href    string
	Active  bool
	Editing bool
	Error   string
}

type ClassesPage struct {
	Shell        Shell
	Year         int
	YearName     string
	Years        []YearOption
	Classes      []ClassRow
	ShowInactive bool
	CanCreate    bool
	ToggleHref   string
	NewName      string
	NewError     string
}

type ClassPage struct {
	Shell    Shell
	ID       int64
	Name     string
	YearName string
	Active   bool
}
