package simulation

// DefaultScenario returns fresh data so requests cannot mutate the shared dataset.
func DefaultScenario() Scenario {
	return Scenario{
		Budget: 100, RequiredDecisions: 5, MaxMeasuresPerCategory: 2, Horizon: 8,
		IndicatorOrder: []Indicator{"T1", "T2", "E1", "E2", "S1", "S2", "B1", "B2", "C1", "C2"},
		IndicatorNames: map[Indicator]string{
			"T1": "Разгрузка дорог",
			"T2": "Доступность общественного транспорта",
			"E1": "Озеленение",
			"E2": "Качество воздуха",
			"S1": "Школы и детсады",
			"S2": "Поликлиники и первичная медпомощь",
			"B1": "Безопасность улиц",
			"B2": "Безопасность дорожного движения",
			"C1": "Надёжность ЖКХ",
			"C2": "Скорость решения обращений жителей",
		},
		CategoryNames: map[Category]string{
			Transport: "Транспорт", Ecology: "Экология", Social: "Соцсфера", Safety: "Безопасность", Services: "Сервисы",
		},
		Weights: Indicators{"T1": .10, "T2": .10, "E1": .09, "E2": .11, "S1": .11, "S2": .11, "B1": .09, "B2": .09, "C1": .10, "C2": .10},
		Scoring: ScoringRules{AverageWeight: .7, MinimumWeight: .3, CriticalThreshold: 40, CriticalPenalty: 1},
		Districts: []District{
			{"esil", "Есиль", .27, values(45, 62, 68, 72, 48, 55, 78, 60, 75, 70)},
			{"almaty", "Алматы", .24, values(40, 75, 50, 55, 60, 65, 62, 52, 50, 60)},
			{"saryarka", "Сарыарка", .20, values(50, 70, 42, 40, 62, 68, 58, 55, 45, 55)},
			{"baikonur", "Байконур", .13, values(52, 68, 55, 50, 58, 60, 52, 58, 55, 58)},
			{"nura", "Нура", .16, values(55, 40, 45, 65, 38, 35, 55, 50, 60, 50)},
		},
		Measures: []Measure{
			{"M1", "Выделенные полосы для автобусов", Transport, DistrictScope, 18, 2, Indicators{"T1": 6, "T2": 9}},
			{"M2", "Умные светофоры (адаптивное управление)", Transport, CityScope, 22, 2, Indicators{"T1": 4, "B2": 3}},
			{"M3", "Линия ЛРТ / расширение", Transport, DistrictScope, 30, 4, Indicators{"T1": 16, "T2": 20, "E2": 4}},
			{"M4", "Парк / сквер", Ecology, DistrictScope, 15, 2, Indicators{"E1": 12, "E2": 3, "B1": 2}},
			{"M5", "Перевод частного сектора на чистое топливо", Ecology, DistrictScope, 25, 3, Indicators{"E2": 14, "C1": 4}},
			{"M6", "Городская программа озеленения и ветрозащитных полос", Ecology, CityScope, 20, 4, Indicators{"E1": 5, "E2": 3}},
			{"M7", "Школа + детсад (модульное строительство)", Social, DistrictScope, 24, 3, Indicators{"S1": 16}},
			{"M8", "Центр семейного здоровья / поликлиника", Social, DistrictScope, 20, 3, Indicators{"S2": 14}},
			{"M9", "Дворовые спорт-хабы", Social, DistrictScope, 10, 1, Indicators{"S1": 3, "S2": 3, "B1": 3}},
			{"M10", "Освещение и камеры (расширение Safe City)", Safety, DistrictScope, 12, 1, Indicators{"B1": 12, "B2": 2}},
			{"M11", "Безопасные переходы и школьные зоны", Safety, DistrictScope, 10, 1, Indicators{"B2": 12, "T1": -2}},
			{"M12", "Единая цифровая платформа обращений", Services, CityScope, 14, 1, Indicators{"C2": 5}},
			{"M13", "Модернизация тепло- и водосетей", Services, DistrictScope, 28, 4, Indicators{"C1": 18, "E2": 2}},
			{"M14", "Аварийные бригады ЖКХ + раннее оповещение", Services, CityScope, 16, 1, Indicators{"C1": 5, "C2": 2}},
		},
		Synergies: []Synergy{
			{[2]string{"M1", "M2"}, "M1", Indicators{"T1": 2}},
			{[2]string{"M10", "M12"}, "M10", Indicators{"B1": 2}},
			{[2]string{"M5", "M6"}, "M5", Indicators{"E2": 2}},
		},
		Incompatibilities: []Incompatibility{
			{[2]string{"M1", "M3"}, false},
			{[2]string{"M4", "M7"}, true},
			{[2]string{"M5", "M13"}, true},
		},
	}
}

func values(t1, t2, e1, e2, s1, s2, b1, b2, c1, c2 float64) Indicators {
	return Indicators{"T1": t1, "T2": t2, "E1": e1, "E2": e2, "S1": s1, "S2": s2, "B1": b1, "B2": b2, "C1": c1, "C2": c2}
}
