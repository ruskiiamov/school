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
	IconAlert     Icon = "alert"
	IconPlus      Icon = "plus"
	IconUp        Icon = "up"
	IconDown      Icon = "down"
	IconKey       Icon = "key"
	IconSwap      Icon = "swap"
	IconCheck     Icon = "check"
	IconEye       Icon = "eye"
	IconEyeOff    Icon = "eye-off"
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
	Href  string
}

type LoginPage struct {
	SchoolName string
	Login      string
	Error      string
}

type HomePage struct {
	Shell     Shell
	Today     string
	YearName  string
	NoClasses bool
	Stats     []Stat
	Section   NavItem
}

type PasswordPage struct {
	Shell  Shell
	Values map[string]string
	Errors map[string]string
	Done   bool
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

type MemberRow struct {
	ID           int64
	FullName     string
	ClassName    string
	Active       bool
	RemoveAction string
}

type ClassStudentsBlock struct {
	CanEdit    bool
	Students   []MemberRow
	Candidates []Option
	AddAction  string
	Error      string
}

type AssignmentRow struct {
	ID            int64
	Subject       string
	Teacher       string
	SubjectActive bool
	TeacherActive bool
	RemoveAction  string
}

type ClassAssignmentsBlock struct {
	CanEdit     bool
	Assignments []AssignmentRow
	Subjects    []Option
	Teachers    []Option
	AddAction   string
	Errors      map[string]string
}

type ClassPage struct {
	Shell       Shell
	ID          int64
	Name        string
	YearName    string
	Active      bool
	Students    ClassStudentsBlock
	Assignments ClassAssignmentsBlock
}

type Option struct {
	ID       int64
	Name     string
	Selected bool
}

type UserFields struct {
	FullName string
	Login    string
	Classes  []Option
	Errors   map[string]string
}

type ChildrenBlock struct {
	Query        string
	Children     []MemberRow
	Results      []MemberRow
	Searched     bool
	SearchAction string
	Hidden       map[string]string
	AddAction    string
	Error        string
}

type UserRow struct {
	ID         int64
	FullName   string
	Login      string
	ClassName  string
	ChildNames string
	Active     bool
	Editing    bool
	Fields     UserFields
	Children   *ChildrenBlock
}

type UsersPage struct {
	Shell        Shell
	Title        string
	Path         string
	ListQuery    string
	Query        string
	ClassFilter  []Option
	ShowClass    bool
	ShowChildren bool
	ShowInactive bool
	ToggleHref   string
	New          UserFields
	Users        []UserRow
	EmptyMessage string
}

type UserCreatedPage struct {
	Shell     Shell
	Title     string
	FullName  string
	Login     string
	Password  string
	Note      string
	ListHref  string
	ListTitle string
	NewHref   string
	NewTitle  string
}

type PasswordResetRow struct {
	ID        int64
	FullName  string
	Role      string
	ClassName string
	Login     string
	Action    string
}

type PasswordResetPage struct {
	Shell Shell
	Path  string
	Query string
	Users []PasswordResetRow
}

type SubstitutionFields struct {
	Classes   []Option
	Subjects  []Option
	Teachers  []Option
	StartDate string
	EndDate   string
	Errors    map[string]string
}

type SubstitutionRow struct {
	ID            int64
	Class         string
	Subject       string
	Teacher       string
	SubjectActive bool
	TeacherActive bool
	Period        string
	Ended         bool
	Editing       bool
	Fields        SubstitutionFields
}

type SubstitutionsPage struct {
	Shell         Shell
	Substitutions []SubstitutionRow
	ShowEnded     bool
	CanCreate     bool
	New           SubstitutionFields
}

type PairOption struct {
	Value    string
	Name     string
	Selected bool
}

type LessonRow struct {
	Href    string
	Date    string
	Class   string
	Subject string
	Topic   string
}

type JournalPage struct {
	Shell   Shell
	Pairs   []PairOption
	Date    string
	Errors  map[string]string
	Lessons []LessonRow
}

type LessonPage struct {
	Shell       Shell
	Title       string
	Path        string
	Topic       string
	TopicError  string
	CanDelete   bool
	DeleteError string
	Block       LessonBlock
}

type LessonStudentItem struct {
	ID        int64
	Href      string
	FullName  string
	ShortName string
	Summary   string
	Active    bool
	InClass   bool
	Selected  bool
}

type MarkFields struct {
	WorkTypes []Option
	Values    []Option
	Label     string
	Errors    map[string]string
}

type MarkRow struct {
	ID           int64
	WorkType     string
	Value        string
	Label        string
	Editing      bool
	Fields       MarkFields
	Action       string
	EditHref     string
	CancelHref   string
	DeleteAction string
}

type LessonStudentPanel struct {
	ID        int64
	FullName  string
	Active    bool
	InClass   bool
	Marks     []MarkRow
	AddAction string
	New       MarkFields
	Error     string
}

type LessonBlock struct {
	Students []LessonStudentItem
	Selected *LessonStudentPanel
}
