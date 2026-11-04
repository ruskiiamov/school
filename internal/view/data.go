package view

import "strconv"

const (
	PenaltiesPath      = "/penalties"
	DiaryPenaltiesPath = "/diary/penalties"

	groupSchool   = "Школа"
	groupCatalogs = "Справочники"
	groupReading  = "Просмотр"
	groupService  = "Служебное"
)

func NavItems(role, active, penalties string) []NavItem {
	items := []NavItem{{Title: "Главная", Href: "/", Icon: IconDashboard}}

	switch role {
	case "admin":
		items = append(items,
			NavItem{Title: "Классы", Href: "/admin/classes", Icon: IconClasses, Group: groupSchool},
			NavItem{Title: "Учителя", Href: "/admin/teachers", Icon: IconTeachers, Group: groupSchool},
			NavItem{Title: "Ученики", Href: "/admin/students", Icon: IconStudents, Group: groupSchool},
			NavItem{Title: "Родители", Href: "/admin/parents", Icon: IconParents, Group: groupSchool},
			NavItem{Title: "Замены", Href: "/admin/substitutions", Icon: IconSwap, Group: groupSchool},
		)

		if penalties != "" {
			items = append(items, NavItem{Title: penalties, Href: PenaltiesPath, Icon: IconPenalty, Group: groupSchool})
		}

		items = append(items,
			NavItem{Title: "Штрафы", Href: "/admin/penalties", Icon: IconSettings, Group: groupSchool},
			NavItem{Title: "Предметы", Href: "/admin/subjects", Icon: IconSubjects, Group: groupCatalogs},
			NavItem{Title: "Журналы", Href: "/admin/journals", Icon: IconTable, Group: groupReading},
			NavItem{Title: "Дневники", Href: "/admin/diary", Icon: IconDiary, Group: groupReading},
			NavItem{Title: "Сброс пароля", Href: "/admin/password-reset", Icon: IconKey, Group: groupService},
			NavItem{Title: "Резервная копия", Href: "/admin/backup", Icon: IconArchive, Group: groupService},
		)
	case "teacher":
		items = append(items, NavItem{Title: "Типы работ", Href: "/journal/work-types", Icon: IconWorkTypes})

		if penalties != "" {
			items = append(items, NavItem{Title: penalties, Href: PenaltiesPath, Icon: IconPenalty})
		}

		items = append(items, NavItem{Title: "Сменить пароль", Href: "/account/password", Icon: IconKey})
	default:
		items = append(items,
			NavItem{Title: "Дневник", Href: "/diary", Icon: IconDiary},
			NavItem{Title: "Оценки", Href: "/diary/summary", Icon: IconMarks},
		)

		if penalties != "" {
			items = append(items, NavItem{Title: penalties, Href: DiaryPenaltiesPath, Icon: IconPenalty})
		}

		items = append(items, NavItem{Title: "Сменить пароль", Href: "/account/password", Icon: IconKey})
	}

	for i := range items {
		items[i].Active = items[i].Href == active
	}

	return items
}

func AdminStats(classes, students, teachers, subjects int) []Stat {
	return []Stat{
		{Label: "Классы", Value: strconv.Itoa(classes), Icon: IconClasses, Href: "/admin/classes"},
		{Label: "Ученики", Value: strconv.Itoa(students), Icon: IconStudents, Href: "/admin/students"},
		{Label: "Учителя", Value: strconv.Itoa(teachers), Icon: IconTeachers, Href: "/admin/teachers"},
		{Label: "Предметы", Value: strconv.Itoa(subjects), Icon: IconSubjects, Href: "/admin/subjects"},
	}
}

func AdminSetup(counts SetupCounts) []SetupStep {
	steps := []SetupStep{
		{Title: "Предметы", Note: "Что преподают в школе: без предметов не назначить учителей", Href: "/admin/subjects", Done: counts.Subjects > 0},
		{Title: "Классы", Note: "Классы текущего учебного года", Href: "/admin/classes", Done: counts.Classes > 0},
		{Title: "Учителя", Note: "Логин и одноразовый пароль приложение придумает само", Href: "/admin/teachers", Done: counts.Teachers > 0},
		{Title: "Ученики", Note: "Создайте учеников и укажите каждому класс", Href: "/admin/students", Done: counts.Students > 0},
		{Title: "Кто что ведёт", Note: "В карточке класса назначьте учителя на каждый предмет: после этого у учителя появится журнал", Href: "/admin/classes", Done: counts.Assignments > 0},
		{Title: "Родители", Note: "По желанию: родитель видит дневники своих детей", Href: "/admin/parents", Done: counts.Parents > 0, Optional: true},
	}

	for _, step := range steps {
		if !step.Done && !step.Optional {
			return steps
		}
	}

	return nil
}
