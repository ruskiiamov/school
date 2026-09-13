package view

func NavItems(role, active string) []NavItem {
	var items []NavItem

	switch role {
	case "admin":
		items = []NavItem{
			{Title: "Дашборд", Href: "/", Icon: IconDashboard},
			{Title: "Классы", Href: "/admin/classes", Icon: IconClasses},
			{Title: "Предметы", Href: "/admin/subjects", Icon: IconSubjects},
			{Title: "Учителя", Href: "/admin/teachers", Icon: IconTeachers},
			{Title: "Ученики", Href: "/admin/students", Icon: IconStudents},
			{Title: "Родители", Href: "/admin/parents", Icon: IconParents},
			{Title: "Типы работ", Href: "/admin/work-types", Icon: IconWorkTypes},
		}
	case "teacher":
		items = []NavItem{
			{Title: "Дашборд", Href: "/", Icon: IconDashboard},
			{Title: "Журнал", Href: "/journal", Icon: IconMarks},
		}
	default:
		items = []NavItem{
			{Title: "Дашборд", Href: "/", Icon: IconDashboard},
			{Title: "Дневник", Href: "/diary", Icon: IconDiary},
		}
	}

	for i := range items {
		items[i].Active = items[i].Href == active
	}

	return items
}

func PlaceholderStats() []Stat {
	return []Stat{
		{Label: "Классы", Value: "0", Icon: IconClasses},
		{Label: "Ученики", Value: "0", Icon: IconStudents},
		{Label: "Учителя", Value: "0", Icon: IconTeachers},
		{Label: "Предметы", Value: "0", Icon: IconSubjects},
	}
}
