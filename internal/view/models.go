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
	IconTable     Icon = "table"
	IconPencil    Icon = "pencil"
	IconTrash     Icon = "trash"
	IconSave      Icon = "save"
	IconRestore   Icon = "restore"
	IconArchive   Icon = "archive"
	IconClose     Icon = "close"
	IconCalendar  Icon = "calendar"
	IconPenalty   Icon = "penalty"
	IconSettings  Icon = "settings"
)

type User struct {
	ShortName string
	Greeting  string
	Role      string
}

type NavItem struct {
	Title  string
	Href   string
	Icon   Icon
	Group  string
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

type SetupStep struct {
	Title    string
	Note     string
	Href     string
	Done     bool
	Optional bool
}

type SetupCounts struct {
	Subjects    int
	Classes     int
	Teachers    int
	Students    int
	Assignments int
	Parents     int
}

type HomePage struct {
	Shell      Shell
	Today      string
	YearName   string
	Setup      []SetupStep
	Stats      []Stat
	Teacher    *TeacherHome
	Children   []DiaryChild
	Student    *StudentDashboard
	NoChildren bool
}

type TeacherHome struct {
	Path          string
	Years         []Option
	Assigned      []PairCard
	Substitutions []PairCard
}

type PairCard struct {
	Href    string
	Name    string
	Lessons string
	Periods []string
	OwnOnly bool
}

type PairJournalPage struct {
	Shell     Shell
	Title     string
	BackHref  string
	Penalties bool
	Content   PairJournalContent
}

type PairJournalContent struct {
	Periods string
	Lessons string
	New     *NewLessonForm
	Table   JournalTable
}

type LessonCardPage struct {
	Shell Shell
	Card  LessonCard
}

type LessonCard struct {
	Title     string
	BackHref  string
	Author    string
	Action    string
	Topic     string
	Text      string
	Due       string
	Errors    map[string]string
	Files     []LessonCardFile
	CanUpload bool
}

type LessonCardFile struct {
	FileLink
	ID     string
	Remove bool
}

type MarksEntryPage struct {
	Shell Shell
	Entry MarksEntry
}

type MarksEntry struct {
	Title        string
	Action       string
	BackHref     string
	InClass      bool
	Rows         []MarksEntryRow
	Empty        string
	EmptyHref    string
	Absent       bool
	Comment      string
	CommentError string
	Debt         string
}

type MarksEntryRow struct {
	Name         string
	Field        string
	Value        string
	CommentField string
	Comment      string
	Error        string
	Autofocus    bool
	Done         *DoneField
}

type DoneField struct {
	Field  string
	Value  string
	Points string
	Error  string
	Locked string
}

type AdminJournalsPage struct {
	Shell    Shell
	Path     string
	Years    []Option
	Classes  []Option
	Subjects []Option
	Teacher  string
	Content  *PairJournalContent
}

type AdminLessonCard struct {
	Title    string
	BackHref string
	Teacher  string
	Topic    string
	Homework *DiaryHomework
}

type AdminLessonCardPage struct {
	Shell Shell
	Card  AdminLessonCard
}

type AdminMarksEntry struct {
	Title     string
	BackHref  string
	DiaryHref string
	InClass   bool
	Marks     []DiaryMark
	Absent    bool
	Comment   string
}

type AdminMarksEntryPage struct {
	Shell Shell
	Entry AdminMarksEntry
}

type NewLessonForm struct {
	Action string
	Date   string
	Error  string
}

type JournalTable struct {
	Students []JournalStudentHead
	Rows     []JournalRow
}

type JournalStudentHead struct {
	ID        string
	LastName  string
	FirstName string
	FullName  string
	Active    bool
	InClass   bool
	Average   string
	Absences  string
}

type JournalRow struct {
	Anchor       string
	Date         string
	Topic        string
	Homework     bool
	CardHref     string
	MarksHref    string
	CollapseHref string
	Expanded     bool
	Author       string
	Highlight    bool
	Edit         *JournalRowEdit
	Cells        []JournalCell
	Marks        *LessonMarksBlock
}

type LessonMarksBlock struct {
	Lesson  string
	Columns int
	Action  string
	ErrorID string
	Error   string
	Empty   string
	Rows    []LessonMarksRow
	NewType *NewWorkTypeForm
}

type LessonMarksRow struct {
	Name   string
	Active bool
	Done   bool
	Cells  []LessonMarkCell
}

type LessonMarkCell struct {
	ID       string
	Field    string
	Label    string
	Value    string
	Comment  string
	Note     string
	Error    string
	Editable bool
	Percent  bool
}

type NewWorkTypeForm struct {
	Action string
	Name   string
}

type LessonMarksPage struct {
	Shell    Shell
	Title    string
	BackHref string
	Students []LessonMarksStudent
	Block    LessonMarksBlock
	Errors   []string
}

type LessonMarksStudent struct {
	Name string
	Href string
}

type JournalRowEdit struct {
	DateAction  string
	DateValue   string
	DateError   string
	TopicAction string
	TopicID     string
	TopicError  string
}

type JournalCell struct {
	Href    string
	Text    string
	Title   string
	Comment bool
	Penalty bool
}

type StudentDashboard struct {
	DiaryHref string
	MarksHref string
	Lessons   []DiaryLessonItem
	Subjects  []SubjectMarksRow
	Penalty   *PenaltyCard
}

type PenaltyCard struct {
	Title string
	Debt  string
	Href  string
}

type PasswordPage struct {
	Shell  Shell
	Values map[string]string
	Errors map[string]string
	Done   bool
}

type DiaryLessonItem struct {
	ID       int64
	Href     string
	Subject  string
	Teacher  string
	Summary  string
	Homework bool
	Selected bool
}

type DiaryMark struct {
	Value    string
	WorkType string
	Comment  string
	Penalty  string
}

type DiaryLessonPanel struct {
	Title    string
	Teacher  string
	Topic    string
	Marks    []DiaryMark
	Absent   bool
	Comment  string
	Homework *DiaryHomework
}

type DiaryHomework struct {
	Text  string
	Due   string
	Files []FileLink
}

type FileLink struct {
	Name string
	Size string
	Href string
}

type DiaryChild struct {
	Name   string
	Href   string
	Active bool
}

type DiaryPage struct {
	Shell         Shell
	Title         string
	BackHref      string
	SummaryHref   string
	PenaltiesHref string
	PenaltiesName string
	Children      []DiaryChild
	NoChildren    bool
	Date          string
	DateLabel     string
	DateAction    string
	Hidden        map[string]string
	PrevHref      string
	TodayHref     string
	NextHref      string
	Lessons       []DiaryLessonItem
	Selected      *DiaryLessonPanel
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
	ID        int64
	Name      string
	Active    bool
	Penalized bool
	Editing   bool
	CanUp     bool
	CanDown   bool
	Error     string
}

type WorkTypesPage struct {
	Shell        Shell
	Base         string
	Subjects     []SubjectTab
	WorkTypes    []WorkTypeRow
	ShowInactive bool
	Penalties    bool
	NewName      string
	NewPenalized bool
	NewError     string
}

type SubjectTab struct {
	Name   string
	Href   string
	Active bool
}

type YearOption struct {
	Year   int
	Name   string
	Href   string
	Active bool
}

type ClassRow struct {
	ID       int64
	Name     string
	Href     string
	Active   bool
	Students string
	Editing  bool
	Error    string
}

type ClassesPage struct {
	Shell        Shell
	Year         int
	YearName     string
	Years        []YearOption
	Classes      []ClassRow
	ShowInactive bool
	CanCreate    bool
	TransferHref string
	ToggleHref   string
	NewName      string
	NewError     string
}

type TransferStudentRow struct {
	ID           int64
	FullName     string
	Active       bool
	CurrentClass string
	Checked      bool
}

type TransferClassRow struct {
	ID            int64
	Name          string
	NewName       string
	Transfer      bool
	Graduating    bool
	Exists        bool
	Students      []TransferStudentRow
	NameError     string
	StudentsError string
}

type TransferPage struct {
	Shell        Shell
	FromYearName string
	ToYearName   string
	Action       string
	BackHref     string
	Classes      []TransferClassRow
	Error        string
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
	LastName   string
	FirstName  string
	MiddleName string
	Login      string
	Classes    []Option
	Errors     map[string]string
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
	Hint         string
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
}

type PasswordResetRow struct {
	ID        int64
	FullName  string
	Role      string
	ClassName string
	Login     string
	Action    string
}

type BackupPage struct {
	Shell        Shell
	DownloadPath string
	Files        string
}

type PenaltySettingsPage struct {
	Shell   Shell
	Action  string
	Enabled bool
	Name    string
	Unit    string
	Full    string
	Errors  map[string]string
	Done    bool
}

type PenaltiesPage struct {
	Shell   Shell
	Title   string
	Path    string
	Query   string
	Years   []Option
	Classes []Option
	Debtors []DebtorRow
	Empty   string
}

type DebtorRow struct {
	FullName  string
	ClassName string
	Debt      string
	Href      string
}

type PenaltyStudentPage struct {
	Shell    Shell
	Title    string
	BackHref string
	Block    PenaltyStudentBlock
}

type PenaltyStudentBlock struct {
	Form    *WorkoffForm
	History PenaltyHistory
}

type WorkoffForm struct {
	Action  string
	Date    string
	Points  string
	Comment string
	Unit    string
	Errors  map[string]string
}

type PenaltyHistory struct {
	Debt      string
	Unit      string
	Penalties []PenaltyHistoryRow
	Workoffs  []WorkoffRow
}

type PenaltyHistoryRow struct {
	Date     string
	Href     string
	Subject  string
	WorkType string
	Done     string
	Points   string
	Status   string
}

type WorkoffRow struct {
	Date         string
	Points       string
	RecordedBy   string
	Comment      string
	DeleteAction string
	Confirm      string
}

type DiaryPenaltiesPage struct {
	Shell      Shell
	Title      string
	BackHref   string
	Children   []DiaryChild
	NoChildren bool
	History    PenaltyHistory
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

type PeriodForm struct {
	From     string
	To       string
	Label    string
	PrevHref string
	ThisHref string
	NextHref string
}

type MarksPage struct {
	Shell      Shell
	Title      string
	Path       string
	BackHref   string
	Children   []DiaryChild
	NoChildren bool
	Hidden     map[string]string
	Years      []Option
	Period     PeriodForm
	Subjects   []SubjectMarksRow
}

type SubjectMarksRow struct {
	Name     string
	Marks    []MarkChip
	Absences string
	Average  string
}

type MarkChip struct {
	Value string
	Title string
	Href  string
}

type DiaryStudentRow struct {
	FullName  string
	ClassName string
	Href      string
}

type DiarySearchPage struct {
	Shell    Shell
	Path     string
	Query    string
	Years    []Option
	Classes  []Option
	Filtered bool
	Students []DiaryStudentRow
}
