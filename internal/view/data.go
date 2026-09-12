package view

func NavItems(active string) []NavItem {
	items := []NavItem{
		{Title: "Дашборд", Href: "/", Icon: IconDashboard},
		{Title: "Классы", Icon: IconClasses, Disabled: true},
		{Title: "Предметы", Icon: IconSubjects, Disabled: true},
		{Title: "Учителя", Icon: IconTeachers, Disabled: true},
		{Title: "Ученики", Icon: IconStudents, Disabled: true},
		{Title: "Расписание", Icon: IconSchedule, Disabled: true},
		{Title: "Журнал оценок", Icon: IconMarks, Disabled: true},
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
