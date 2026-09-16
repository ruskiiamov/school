package view

import "strconv"

func NavItems(role, active string) []NavItem {
	items := []NavItem{{Title: "Дашборд", Href: "/", Icon: IconDashboard}}

	if role == "admin" {
		items = append(items,
			NavItem{Title: "Классы", Href: "/admin/classes", Icon: IconClasses},
			NavItem{Title: "Предметы", Href: "/admin/subjects", Icon: IconSubjects},
			NavItem{Title: "Учителя", Href: "/admin/teachers", Icon: IconTeachers},
			NavItem{Title: "Ученики", Href: "/admin/students", Icon: IconStudents},
			NavItem{Title: "Родители", Href: "/admin/parents", Icon: IconParents},
			NavItem{Title: "Типы работ", Href: "/admin/work-types", Icon: IconWorkTypes},
			NavItem{Title: "Замены", Href: "/admin/substitutions", Icon: IconSwap},
			NavItem{Title: "Дневники", Href: "/admin/diary", Icon: IconDiary},
			NavItem{Title: "Сброс пароля", Href: "/admin/password-reset", Icon: IconKey},
		)
	} else {
		items = append(items, SectionItem(role), NavItem{Title: "Сменить пароль", Href: "/account/password", Icon: IconKey})
	}

	for i := range items {
		items[i].Active = items[i].Href == active
	}

	return items
}

func SectionItem(role string) NavItem {
	if role == "teacher" {
		return NavItem{Title: "Журнал", Href: "/journal", Icon: IconMarks}
	}

	return NavItem{Title: "Дневник", Href: "/diary", Icon: IconDiary}
}

func AdminStats(classes, students, teachers, subjects int) []Stat {
	return []Stat{
		{Label: "Классы", Value: strconv.Itoa(classes), Icon: IconClasses, Href: "/admin/classes"},
		{Label: "Ученики", Value: strconv.Itoa(students), Icon: IconStudents, Href: "/admin/students"},
		{Label: "Учителя", Value: strconv.Itoa(teachers), Icon: IconTeachers, Href: "/admin/teachers"},
		{Label: "Предметы", Value: strconv.Itoa(subjects), Icon: IconSubjects, Href: "/admin/subjects"},
	}
}
