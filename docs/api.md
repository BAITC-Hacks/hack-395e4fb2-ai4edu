# HTTP API «Аким на 5 часов»

Контракт текущего сервера для фронтенда и AI-участника. Источники:
`internal/httpapi/{handler,recommend,cors}.go`, `internal/optimizer/optimizer.go`,
`internal/simulation/{models,dataset,validator,decision_json,engine}.go`
и `internal/explanation/service.go`. Правила расчёта: [scoring.md](scoring.md).
Все пути относительно адреса сервера (по умолчанию `http://localhost:8080`).

## Эндпоинты

| Метод | Путь | Назначение и успешный ответ |
|---|---|---|
| GET | `/api/scenario` | 200: каталог, исходные данные и правила, объект Scenario |
| HEAD | `/api/scenario` | Автоматически поддерживается Go ServeMux как GET, по HTTP без тела |
| POST | `/api/simulate` | 200: валидация и расчёт, объект Result |
| POST | `/api/explain` | 200: расчёт и объяснение, объект с `result` и `explanation` |
| POST | `/api/recommend` | 200: глобальный оптимум (`mode=best`) или до трёх улучшений (`mode=improve`) |
| OPTIONS | любой путь | 204, пустое тело; CORS middleware обрабатывает до маршрутизации |

Параметров URL нет. GET не требует тела. POST использует JSON-объект запроса
ниже; клиенту следует посылать `Content-Type: application/json` (сервер сам
этот заголовок не проверяет). JSON-ответы имеют
`Content-Type: application/json; charset=utf-8` и завершающий перевод строки.
Неизвестный путь, кроме OPTIONS, возвращает стандартный 404 Go ServeMux,
`text/plain; charset=utf-8`, тело `404 page not found\n`.

## Запрос POST /api/simulate и POST /api/explain

| Поле | Тип JSON | Обязательность и смысл |
|---|---|---|
| `decisions` | array of Decision | Обязательно для валидного сценария: ровно 5 решений. Отсутствие, `null`, `[]` → 422 `decision_count` |
| `decisions[].measure_id` | string | Обязательно: известный ID M1–M14; отсутствие, `null` или пустая строка дают 422 `unknown_measure` |
| `decisions[].district_id` | string | Для меры `District` обязательно: известный ID района. Отсутствие, `null`, пустая строка → 422 `district_required` |

Для меры `City` поле `district_id` **должно отсутствовать целиком**:
даже `null` и `""` запрещены (422 `city_district_forbidden`). Строки ID
чувствительны к регистру. Повторять одну меру в разных районах нельзя.
Нужно ровно 5 уникальных мер, стоимость ≤ 100, максимум 2 меры одного
направления; несовместимости описаны в Scenario и таблице ошибок.

Тело ограничено 65 536 байтами (64 КиБ), включая пробелы. При обнаружении
превышения лимита чтения → 413; если раньше обнаружена ошибка JSON, возможен 400.
Требуется ровно один JSON-объект: пустое тело, верхнеуровневый `null`, массив,
неверные типы, неизвестные поля (в том числе внутри Decision), мусор или
второе значение после объекта → 400. Собственные Score/Result передавать нельзя.
Синтаксически допустимые `null` внутри массива решений декодируются как пустое
решение и дают `unknown_measure`.

Текущий декодер `encoding/json` принимает имена полей без учёта регистра
(например, `Decisions`, `MEASURE_ID`). Повторяющиеся ключи принимаются;
обычно позднее значение заменяет раннее. Особенность `measure_id`: поздний
`null` не стирает ранее прочитанную строку. Это текущее поведение, строгий
разбор регистра и дублей пока не реализован.

## Типы и единицы

`integer` ниже — целое JSON number; `number` — JSON number, вычисляемый в Go
как float64. Все показатели — нормированные баллы [0, 100], больше значит
лучше; эффекты и дельты — пункты той же шкалы, могут быть отрицательными.
Score — баллы формулы из scoring.md; стоимость — условные единицы бюджета,
время — кварталы. Доли и веса безразмерны (0.27 = 27%).

`Indicators` — object с ключами ID показателей и значениями number.
Полные показатели, веса и дельты содержат все 10 ключей; эффекты мер и синергий
содержат только затронутые показатели. `indicator_order` задаёт порядок
показателей; порядок ключей JSON-объекта не нужен для отображения.

## 200: Scenario (GET /api/scenario)

Все перечисленные поля присутствуют, `null` не используется.

| Поле | Тип | Смысл / единицы |
|---|---|---|
| `budget` | integer | Лимит стоимости: 100 условных единиц |
| `required_decisions` | integer | Требуемое число решений: 5 |
| `max_measures_per_category` | integer | Максимум мер направления: 2 |
| `horizon_quarters` | integer | Горизонт расчёта: 8 кварталов |
| `indicator_order` | array of string | Порядок 10 ID показателей |
| `indicator_names` | object: ID → string | Русские названия показателей |
| `category_names` | object: ID → string | Русские названия направлений |
| `indicator_weights` | Indicators | Веса показателей, сумма 1 |
| `scoring` | object | Параметры городской формулы, см. ниже |
| `scoring.average_weight` | number | Вес среднего районного Score: 0.7 |
| `scoring.minimum_weight` | number | Вес минимального районного Score: 0.3 |
| `scoring.critical_threshold` | number | Критический порог: 40 баллов; сравнение строго `<` |
| `scoring.critical_penalty` | number | Штраф: 1 балл Score за каждую критическую пару район × показатель |
| `districts` | array of District | 5 районов, порядок: esil, almaty, saryarka, baikonur, nura |
| `districts[].id` | string | ID района |
| `districts[].name` | string | Название района |
| `districts[].population_share` | number | Доля населения, сумма по районам 1 |
| `districts[].indicators` | Indicators | Исходные значения, баллы [0, 100] |
| `measures` | array of Measure | 14 мер, порядок M1…M14 |
| `measures[].id` | string | ID меры |
| `measures[].name` | string | Название меры |
| `measures[].category` | string | Направление: Transport, Ecology, Social, Safety, Services |
| `measures[].scope` | string | `District` — один район; `City` — все районы |
| `measures[].cost` | integer | Стоимость в условных единицах |
| `measures[].lag` | integer | Лаг начала действия, кварталы |
| `measures[].effects` | Indicators | Полные эффекты до умножения на лаг, пункты показателей |
| `synergies` | array of Synergy | Бонусы при выборе обеих мер пары |
| `synergies[].measure_ids` | array of 2 strings | Пара ID мер |
| `synergies[].district_measure_id` | string | ID меры пары, чей выбранный район получает бонус |
| `synergies[].effects` | Indicators | Бонус в пунктах, без масштабирования лагом, до clipping |
| `incompatibilities` | array of Incompatibility | Запрещённые сочетания |
| `incompatibilities[].measure_ids` | array of 2 strings | Пара несовместимых ID |
| `incompatibilities[].same_district_only` | boolean | true: запрет только в одном районе; false: запрет во всём городе |

## 200: Result (POST /api/simulate)

Все поля таблицы присутствуют в успешном ответе. `validation_errors`
отсутствует. Решения сортируются лексикографически по `measure_id`, затем
`district_id` (M10 раньше M5); у городских решений `district_id` отсутствует.

| Поле | Тип | Смысл / единицы |
|---|---|---|
| `valid` | boolean | Всегда true для 200 |
| `total_cost` | integer | Суммарная стоимость, условные единицы |
| `remaining_budget` | integer | 100 − total_cost, условные единицы; бонуса за остаток нет |
| `decisions` | array of Decision | Канонический список решений; поля как в запросе |
| `base_score` | number | Городской Score до мер: 52.55768 с точностью float64 |
| `final_score` | number | Городской Score после мер, синергий и clipping |
| `score_delta` | number | final_score − base_score, баллы Score |
| `critical_before` | integer | Число пар район × показатель со значением < 40 до мер: 2 |
| `critical_after` | integer | Такое же число после всех изменений |
| `district_before_after` | array of DistrictResult | Все районы в порядке Scenario.districts |
| `district_before_after[].district_id` | string | ID района |
| `district_before_after[].name` | string | Название района |
| `district_before_after[].population_share` | number | Доля населения |
| `district_before_after[].before` | Indicators | Исходные значения показателей, баллы |
| `district_before_after[].after` | Indicators | Значения после всех эффектов, синергий и clipping в [0, 100] |
| `district_before_after[].score_before` | number | Взвешенная сумма исходных показателей, районные баллы |
| `district_before_after[].score_after` | number | Взвешенная сумма итоговых показателей, районные баллы |
| `indicator_deltas` | object: district ID → Indicators | **После clipping**: after − before для всех районов и показателей, пункты |
| `applied_effects` | array of AppliedEffect | **До clipping**: реализованные эффекты каждой меры по целевым районам |
| `applied_effects[].measure_id` | string | ID применённой меры |
| `applied_effects[].district_id` | string | ID целевого района; городская мера создаёт 5 записей |
| `applied_effects[].lag_factor` | number | Безразмерный множитель (8 − lag) / 8 |
| `applied_effects[].effects` | Indicators | Эффект уже умножен на lag_factor, пункты; без синергии и до clipping |
| `applied_synergies` | array of AppliedSynergy | Применённые бонусы в порядке Scenario.synergies; `[]`, если нет |
| `applied_synergies[].measure_ids` | array of 2 strings | Пара ID мер |
| `applied_synergies[].district_id` | string | ID района бонуса |
| `applied_synergies[].effects` | Indicators | Пункты бонуса до clipping; лаг не применяется |
| `base_breakdown` | ScoreBreakdown | Компоненты Score до мер |
| `final_breakdown` | ScoreBreakdown | Компоненты Score после изменений |
| `base_breakdown.weighted_average`, `final_breakdown.weighted_average` | number | Среднее районных Score с весами населения, баллы; ещё без множителя 0.7 |
| `base_breakdown.minimum_district`, `final_breakdown.minimum_district` | number | Минимальный районный Score, баллы; ещё без множителя 0.3 |
| `base_breakdown.critical_penalty`, `final_breakdown.critical_penalty` | number | Полный вычитаемый штраф, баллы Score |

Итог = 0.7 × weighted_average + 0.3 × minimum_district − critical_penalty.
Нельзя просто складывать `applied_effects` для получения фактических изменений:
нужно учитывать синергии и clipping; готовое изменение даёт `indicator_deltas`.
Числа сериализуются без прикладного округления. Golden Score ≈ 56.54307;
длинные десятичные хвосты float64 в примере ниже ожидаемы.

## 200: POST /api/explain

| Поле | Тип | Смысл |
|---|---|---|
| `result` | Result | Полный объект выше; совпадает с /api/simulate для того же запроса |
| `explanation` | object | Текстовое объяснение готового расчёта |
| `explanation.source` | string | `llm` при успешной генерации; иначе `fallback` |
| `explanation.text` | string | Непустой текст объяснения; отображать как текст, без HTML-интерпретации |

Без ключа, при ошибке/таймауте провайдера или пустом ответе возвращается 200
с `source="fallback"`. AI не меняет числа движка. Невалидный запрос не вызывает
AI: 400/413/422 возвращаются без обёртки `result` и без `explanation`.
На границе AI сервис передаёт сериализованный JSON с `result`,
`selected_measures` (полные Measure), `indicator_names`, `category_names`,
`critical_threshold` и `horizon_quarters`. Это внутренний вход клиента AI,
не дополнительный HTTP-эндпоинт.

Движок и JSON `/api/scenario`, `/api/simulate` детерминированы: одинаковый вход
даёт побайтово одинаковый JSON; перестановка решений сохраняет ответ симуляции.
У `/api/explain` детерминированы `result` и fallback. Внешний LLM может менять
текст между вызовами; текущий код не гарантирует побайтовое равенство этой части.

## Ошибки

| HTTP | Причина | Тело |
|---|---|---|
| 400 | Ошибка декодирования/формы JSON | Только `valid:false`, `validation_errors` с `invalid_json` |
| 413 | Превышен лимит чтения тела | Только `valid:false`, `validation_errors` с `request_too_large` |
| 422 | Нарушение правил сценария | Невалидный Result: поля ниже, без расчёта Score |
| 405 | Неподдерживаемый метод для известного пути | `text/plain; charset=utf-8`, `Method Not Allowed\n`; это **не JSON** |

405 выставляет `Allow: GET, HEAD` для `/api/scenario`, `Allow: POST` для
`/api/simulate`, `/api/explain` и `/api/recommend`. OPTIONS перехватывается до этой проверки.

422 содержит `valid` (boolean, false), `total_cost` (integer, сумма стоимостей
известных мер, **включая повторы**), `remaining_budget` (integer, 100 − стоимость,
может быть отрицательным), `decisions` (канонически отсортированный array),
`validation_errors` (непустой array), `applied_synergies` (пустой array).
Неизвестные меры не имеют стоимости. Все поля Score, critical, breakdown,
district_before_after, indicator_deltas и applied_effects отсутствуют.
В возвращённых решениях `district_id:null` опускается при сериализации:
наличие поля в исходном запросе учитывается валидатором, но не сохраняется в ответе.

Каждый элемент `validation_errors`:

| Поле | Тип | Обязательность / смысл |
|---|---|---|
| `code` | string | Всегда; машинный код из таблицы ниже |
| `message` | string | Всегда; английское описание. Клиенту следует ветвиться по code, а не тексту |
| `measure_ids` | array of string | Только при связи с конкретными мерами; иначе поле отсутствует |

Валидатор собирает все найденные нарушения. Порядок детерминирован: количество,
ошибки канонического списка решений, бюджет, направления в порядке
Transport/Ecology/Social/Safety/Services, несовместимости в порядке каталога.

Полный список `code` (примеры показывают конкретное нарушение, вместе с ним
могут возникнуть другие ошибки):

| code | HTTP | Ситуация-пример | measure_ids |
|---|---|---|---|
| `invalid_json` | 400 | Оборванное тело `{"decisions":`; неизвестное поле `score` | Нет |
| `request_too_large` | 413 | 65 536 пробелов перед корректным объектом | Нет |
| `decision_count` | 422 | Четыре решения вместо пяти | Нет |
| `duplicate_measure` | 422 | M7 выбран дважды, даже в разных районах | Повторённый ID |
| `unknown_measure` | 422 | measure_id = M99 (или пустой ID) | Неизвестный ID |
| `city_district_forbidden` | 422 | M12 с district_id = null, пустой строкой или nura | ID городской меры |
| `district_required` | 422 | M7 без district_id, с null или пустой строкой | ID районной меры |
| `unknown_district` | 422 | M7 с district_id = unknown | ID меры |
| `budget_exceeded` | 422 | M3 + M5 + M7 + M8 + M13 стоят 127 | Нет |
| `category_limit` | 422 | M7 + M8 + M9: три меры Social | Нет |
| `incompatible_measures` | 422 | M1 + M3 в любых районах; M4 + M7 в одном районе; M5 + M13 в одном районе | Оба ID пары |

## POST /api/recommend

Эндпоинт возвращает решения и числа движка, без обращения к AI и без
генерации текстов. Существующие `/api/scenario`, `/api/simulate`, `/api/explain`
сохраняют свои запросы и ответы. Общие правила JSON, лимит 64 КиБ, CORS,
OPTIONS и формат ошибок распространяются и на `/api/recommend`.

### Запрос

| Поле | Тип JSON | Обязательность / смысл |
|---|---|---|
| `mode` | string | Обязательно, строго `best` или `improve` (значение чувствительно к регистру) |
| `decisions` | array of Decision | В режиме `improve` — исходные решения, все правила как у /api/simulate |

`best`: поле `decisions` должно отсутствовать, даже `null` запрещён.
`improve`: отсутствие `decisions`, `null` и пустой массив дают 422
`decision_count`. Для городских мер `district_id` должен отсутствовать,
для районных обязателен. Передавать собственные оценки нельзя.

Неизвестный/отсутствующий/null `mode`, `decisions` в режиме `best`,
неверный тип, неизвестное поле или битый JSON → 400 `invalid_json`.
Превышение лимита чтения тела → 413 `request_too_large`.
Невалидный сценарий в `improve` → 422 с **тем же телом**, что у
`/api/simulate` для его `decisions`, без обёртки и без рекомендаций.
Коды валидатора не добавлялись. GET/HEAD → 405 с `Allow: POST`;
OPTIONS → 204 по общим правилам CORS.

### 200: best

| Поле | Тип | Смысл / единицы |
|---|---|---|
| `best` | Candidate | Глобально лучший валидный сценарий |
| `best.decisions` | array of Decision | Ровно 5 решений в каноническом порядке движка |
| `best.final_score` | number | Итоговый городской Score, баллы |
| `best.total_cost` | integer | Стоимость, условные единицы |
| `best.remaining_budget` | integer | Остаток бюджета, условные единицы |
| `best.critical_after` | integer | Число критических пар район × показатель после изменений |

Перебираются все сочетания 5 уникальных мер из 14 и все назначения районов
районным мерам. По бюджету, лимиту направлений и несовместимостям из каталога
ветви отсекаются до расчёта. Каждый оставшийся полный сценарий проверяется и
оценивается исключительно `simulation.Simulate`; отдельной формулы нет.
На текущем датасете остаётся 694 395 сценариев.

Поиск выполняется один раз на процесс через `sync.Once`. `cmd/server`
завершает его **до начала прослушивания HTTP**; прогресс старта и время поиска
пишутся в лог. Хранятся только выбранные решения; каждый вызов Best заново
получает из движка независимый результат для этих решений. Перезапуск процесса
повторяет поиск. При использовании HTTP handler вне `cmd/server` без прогрева
первый вызов `best` сам выполняет поиск, остальные ждут тот же `sync.Once`.

Для ориентира: при локальной проверке поиск при старте занял 17.4 с,
последующий HTTP best — 1.7 мс; время зависит от оборудования. В ответ время
не включается, чтобы сохранить побайтовый детерминизм.

### 200: improve

| Поле | Тип | Смысл / единицы |
|---|---|---|
| `current_score` | number | Score присланного валидного сценария, баллы |
| `improvements` | array of Improvement | От 0 до 3 лучших строго улучшающих замен; `[]`, если их нет |
| `improvements[].decisions` | array of Decision | Полный новый набор из 5 решений в каноническом порядке |
| `improvements[].final_score` | number | Новый городской Score, баллы |
| `improvements[].score_delta` | number | final_score − current_score, строго положительное число баллов |
| `improvements[].total_cost` | integer | Полная стоимость нового набора, условные единицы |
| `improvements[].remaining_budget` | integer | Остаток бюджета нового набора, условные единицы |
| `improvements[].critical_after` | integer | Число критических пар после изменений |

Одна замена означает замену **одного решения**, остальные четыре сохраняются:
либо та же мера получает другой район, либо выбирается другая мера с любым
допустимым районом (городская — без района). Повторы мер запрещены и здесь.
Рассматриваются все такие соседи; валидность и Score каждого определяет
`simulation.Simulate`. Равные исходному Score варианты не являются улучшениями.
Если улучшений меньше трёх, возвращаются все найденные, без заполнения массива
повторами. Глобальный оптимум имеет пустой список улучшений; пустой список
у другого набора означает лишь отсутствие улучшений одной заменой.

Оба режима сравнивают float64 без округления и допуска: сначала больший Score,
при точном равенстве — лексикографически меньшая последовательность пар
`(measure_id, district_id)` канонического списка решений. Например, M10 раньше
M2, almaty раньше nura. Improve сортируется по этому же правилу. Его дельта
считается **от присланного сценария**, в отличие от `Result.score_delta`,
который отсчитывается от базового города.

Одинаковый вход → побайтово одинаковый JSON; перестановка решений в improve
также сохраняет ответ. Для текущего набора данных лучший Score ≈ 57.236735,
стоимость 98, критических значений 0. Golden-инварианты остаются прежними:
base ≈ 52.55768, golden ≈ 56.54307.

## CORS

`CORS_ORIGIN` — единственный разрешённый origin, например
`http://localhost:5173`; значение берётся из окружения при старте сервера.
При непустом значении, отличном от `*`, каждый ответ получает `Vary: Origin`.
Только точное совпадение заголовка `Origin` с настройкой добавляет
`Access-Control-Allow-Origin` с этим значением. Это относится и к ошибкам.
Пустая настройка или `*` отключают разрешающие CORS-заголовки.

OPTIONS для **любого пути**, даже неизвестного, возвращает 204 без тела.
Всегда добавляются `Vary: Access-Control-Request-Method` и
`Vary: Access-Control-Request-Headers`. При совпавшем Origin дополнительно:
`Access-Control-Allow-Methods: GET, POST, OPTIONS` и
`Access-Control-Allow-Headers: Content-Type`. Credentials и Max-Age не задаются.
Middleware не проверяет запрошенные preflight метод/заголовки, а возвращает
этот фиксированный список. Несовпавший Origin не блокирует обработку запроса
на сервере, но не получает разрешающих заголовков для браузера.

## Справочник ID

Названия взяты из `DefaultScenario()`, доступны также в GET /api/scenario.

| ID района | Название |
|---|---|
| esil | Есиль |
| almaty | Алматы |
| saryarka | Сарыарка |
| baikonur | Байконур |
| nura | Нура |

| ID меры | Название | scope |
|---|---|---|
| M1 | Выделенные полосы для автобусов | District |
| M2 | Умные светофоры (адаптивное управление) | City |
| M3 | Линия ЛРТ / расширение | District |
| M4 | Парк / сквер | District |
| M5 | Перевод частного сектора на чистое топливо | District |
| M6 | Городская программа озеленения и ветрозащитных полос | City |
| M7 | Школа + детсад (модульное строительство) | District |
| M8 | Центр семейного здоровья / поликлиника | District |
| M9 | Дворовые спорт-хабы | District |
| M10 | Освещение и камеры (расширение Safe City) | District |
| M11 | Безопасные переходы и школьные зоны | District |
| M12 | Единая цифровая платформа обращений | City |
| M13 | Модернизация тепло- и водосетей | District |
| M14 | Аварийные бригады ЖКХ + раннее оповещение | City |

| ID направления | Название |
|---|---|
| Transport | Транспорт |
| Ecology | Экология |
| Social | Соцсфера |
| Safety | Безопасность |
| Services | Сервисы |

| ID показателя | Название |
|---|---|
| T1 | Разгрузка дорог |
| T2 | Доступность общественного транспорта |
| E1 | Озеленение |
| E2 | Качество воздуха |
| S1 | Школы и детсады |
| S2 | Поликлиники и первичная медпомощь |
| B1 | Безопасность улиц |
| B2 | Безопасность дорожного движения |
| C1 | Надёжность ЖКХ |
| C2 | Скорость решения обращений жителей |


## Полные примеры из запущенного сервера

Тела ответов получены HTTP-запросами к `go run ./cmd/server`,
без API-ключа, и отформатированы с отступами без изменения данных.
Во всех трёх случаях: `POST /api/simulate`, запрос с
`Content-Type: application/json`, ответ с
`Content-Type: application/json; charset=utf-8`. Для `/api/explain`
те же ошибки возвращаются без изменений; успешный Result оборачивается
в `result`, как описано выше.

### Golden: HTTP 200

M7, M8, M10 → nura; M12 → город; M5 → saryarka.

Запрос:

<!-- example:golden-request -->
```json
{
  "decisions": [
    {
      "measure_id": "M7",
      "district_id": "nura"
    },
    {
      "measure_id": "M8",
      "district_id": "nura"
    },
    {
      "measure_id": "M10",
      "district_id": "nura"
    },
    {
      "measure_id": "M12"
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka"
    }
  ]
}
```

Ответ (200):

<!-- example:golden-response -->
```json
{
  "valid": true,
  "total_cost": 95,
  "remaining_budget": 5,
  "decisions": [
    {
      "measure_id": "M10",
      "district_id": "nura"
    },
    {
      "measure_id": "M12"
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka"
    },
    {
      "measure_id": "M7",
      "district_id": "nura"
    },
    {
      "measure_id": "M8",
      "district_id": "nura"
    }
  ],
  "base_score": 52.557680000000005,
  "final_score": 56.54306999999999,
  "score_delta": 3.985389999999988,
  "critical_before": 2,
  "critical_after": 0,
  "district_before_after": [
    {
      "district_id": "esil",
      "name": "Есиль",
      "population_share": 0.27,
      "before": {
        "B1": 78,
        "B2": 60,
        "C1": 75,
        "C2": 70,
        "E1": 68,
        "E2": 72,
        "S1": 48,
        "S2": 55,
        "T1": 45,
        "T2": 62
      },
      "after": {
        "B1": 78,
        "B2": 60,
        "C1": 75,
        "C2": 74.375,
        "E1": 68,
        "E2": 72,
        "S1": 48,
        "S2": 55,
        "T1": 45,
        "T2": 62
      },
      "score_before": 62.99,
      "score_after": 63.4275
    },
    {
      "district_id": "almaty",
      "name": "Алматы",
      "population_share": 0.24,
      "before": {
        "B1": 62,
        "B2": 52,
        "C1": 50,
        "C2": 60,
        "E1": 50,
        "E2": 55,
        "S1": 60,
        "S2": 65,
        "T1": 40,
        "T2": 75
      },
      "after": {
        "B1": 62,
        "B2": 52,
        "C1": 50,
        "C2": 64.375,
        "E1": 50,
        "E2": 55,
        "S1": 60,
        "S2": 65,
        "T1": 40,
        "T2": 75
      },
      "score_before": 57.059999999999995,
      "score_after": 57.497499999999995
    },
    {
      "district_id": "saryarka",
      "name": "Сарыарка",
      "population_share": 0.2,
      "before": {
        "B1": 58,
        "B2": 55,
        "C1": 45,
        "C2": 55,
        "E1": 42,
        "E2": 40,
        "S1": 62,
        "S2": 68,
        "T1": 50,
        "T2": 70
      },
      "after": {
        "B1": 58,
        "B2": 55,
        "C1": 47.5,
        "C2": 59.375,
        "E1": 42,
        "E2": 48.75,
        "S1": 62,
        "S2": 68,
        "T1": 50,
        "T2": 70
      },
      "score_before": 54.650000000000006,
      "score_after": 56.3
    },
    {
      "district_id": "baikonur",
      "name": "Байконур",
      "population_share": 0.13,
      "before": {
        "B1": 52,
        "B2": 58,
        "C1": 55,
        "C2": 58,
        "E1": 55,
        "E2": 50,
        "S1": 58,
        "S2": 60,
        "T1": 52,
        "T2": 68
      },
      "after": {
        "B1": 52,
        "B2": 58,
        "C1": 55,
        "C2": 62.375,
        "E1": 55,
        "E2": 50,
        "S1": 58,
        "S2": 60,
        "T1": 52,
        "T2": 68
      },
      "score_before": 56.629999999999995,
      "score_after": 57.067499999999995
    },
    {
      "district_id": "nura",
      "name": "Нура",
      "population_share": 0.16,
      "before": {
        "B1": 55,
        "B2": 50,
        "C1": 60,
        "C2": 50,
        "E1": 45,
        "E2": 65,
        "S1": 38,
        "S2": 35,
        "T1": 55,
        "T2": 40
      },
      "after": {
        "B1": 67.5,
        "B2": 51.75,
        "C1": 60,
        "C2": 54.375,
        "E1": 45,
        "E2": 65,
        "S1": 48,
        "S2": 43.75,
        "T1": 55,
        "T2": 40
      },
      "score_before": 49.18000000000001,
      "score_after": 52.962500000000006
    }
  ],
  "indicator_deltas": {
    "almaty": {
      "B1": 0,
      "B2": 0,
      "C1": 0,
      "C2": 4.375,
      "E1": 0,
      "E2": 0,
      "S1": 0,
      "S2": 0,
      "T1": 0,
      "T2": 0
    },
    "baikonur": {
      "B1": 0,
      "B2": 0,
      "C1": 0,
      "C2": 4.375,
      "E1": 0,
      "E2": 0,
      "S1": 0,
      "S2": 0,
      "T1": 0,
      "T2": 0
    },
    "esil": {
      "B1": 0,
      "B2": 0,
      "C1": 0,
      "C2": 4.375,
      "E1": 0,
      "E2": 0,
      "S1": 0,
      "S2": 0,
      "T1": 0,
      "T2": 0
    },
    "nura": {
      "B1": 12.5,
      "B2": 1.75,
      "C1": 0,
      "C2": 4.375,
      "E1": 0,
      "E2": 0,
      "S1": 10,
      "S2": 8.75,
      "T1": 0,
      "T2": 0
    },
    "saryarka": {
      "B1": 0,
      "B2": 0,
      "C1": 2.5,
      "C2": 4.375,
      "E1": 0,
      "E2": 8.75,
      "S1": 0,
      "S2": 0,
      "T1": 0,
      "T2": 0
    }
  },
  "applied_effects": [
    {
      "measure_id": "M10",
      "district_id": "nura",
      "lag_factor": 0.875,
      "effects": {
        "B1": 10.5,
        "B2": 1.75
      }
    },
    {
      "measure_id": "M12",
      "district_id": "esil",
      "lag_factor": 0.875,
      "effects": {
        "C2": 4.375
      }
    },
    {
      "measure_id": "M12",
      "district_id": "almaty",
      "lag_factor": 0.875,
      "effects": {
        "C2": 4.375
      }
    },
    {
      "measure_id": "M12",
      "district_id": "saryarka",
      "lag_factor": 0.875,
      "effects": {
        "C2": 4.375
      }
    },
    {
      "measure_id": "M12",
      "district_id": "baikonur",
      "lag_factor": 0.875,
      "effects": {
        "C2": 4.375
      }
    },
    {
      "measure_id": "M12",
      "district_id": "nura",
      "lag_factor": 0.875,
      "effects": {
        "C2": 4.375
      }
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka",
      "lag_factor": 0.625,
      "effects": {
        "C1": 2.5,
        "E2": 8.75
      }
    },
    {
      "measure_id": "M7",
      "district_id": "nura",
      "lag_factor": 0.625,
      "effects": {
        "S1": 10
      }
    },
    {
      "measure_id": "M8",
      "district_id": "nura",
      "lag_factor": 0.625,
      "effects": {
        "S2": 8.75
      }
    }
  ],
  "applied_synergies": [
    {
      "measure_ids": [
        "M10",
        "M12"
      ],
      "district_id": "nura",
      "effects": {
        "B1": 2
      }
    }
  ],
  "base_breakdown": {
    "weighted_average": 56.8624,
    "minimum_district": 49.18000000000001,
    "critical_penalty": 2
  },
  "final_breakdown": {
    "weighted_average": 58.07759999999999,
    "minimum_district": 52.962500000000006,
    "critical_penalty": 0
  }
}
```

### Невалидный сценарий: HTTP 422

Тот же набор, но у городской M12 передан district_id: null.

Запрос:

<!-- example:invalid-request -->
```json
{
  "decisions": [
    {
      "measure_id": "M7",
      "district_id": "nura"
    },
    {
      "measure_id": "M8",
      "district_id": "nura"
    },
    {
      "measure_id": "M10",
      "district_id": "nura"
    },
    {
      "measure_id": "M12",
      "district_id": null
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka"
    }
  ]
}
```

Ответ (422):

<!-- example:invalid-response -->
```json
{
  "valid": false,
  "total_cost": 95,
  "remaining_budget": 5,
  "decisions": [
    {
      "measure_id": "M10",
      "district_id": "nura"
    },
    {
      "measure_id": "M12"
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka"
    },
    {
      "measure_id": "M7",
      "district_id": "nura"
    },
    {
      "measure_id": "M8",
      "district_id": "nura"
    }
  ],
  "validation_errors": [
    {
      "code": "city_district_forbidden",
      "message": "District must be omitted for a city measure",
      "measure_ids": [
        "M12"
      ]
    }
  ],
  "applied_synergies": []
}
```

### Битый JSON: HTTP 400

Тело обрывается сразу после двоеточия; блок запроса намеренно невалиден.

Запрос:

<!-- example:malformed-request -->
```json
{"decisions":
```

Ответ (400):

<!-- example:malformed-response -->
```json
{
  "valid": false,
  "validation_errors": [
    {
      "code": "invalid_json",
      "message": "unexpected EOF"
    }
  ]
}
```

## Реальные примеры /api/recommend

Оба ответа получены от запущенного сервера через HTTP и отформатированы
без изменения данных; тест сверяет их с текущим обработчиком.
Запросы: `POST /api/recommend`, `Content-Type: application/json`.
Оба ответа: HTTP 200, `Content-Type: application/json; charset=utf-8`.

### Глобальный оптимум

Запрос:

<!-- example:recommend-best-request -->
```json
{
  "mode": "best"
}
```

Ответ:

<!-- example:recommend-best-response -->
```json
{
  "best": {
    "decisions": [
      {
        "measure_id": "M14"
      },
      {
        "measure_id": "M2"
      },
      {
        "measure_id": "M3",
        "district_id": "nura"
      },
      {
        "measure_id": "M8",
        "district_id": "nura"
      },
      {
        "measure_id": "M9",
        "district_id": "nura"
      }
    ],
    "final_score": 57.236734999999996,
    "total_cost": 98,
    "remaining_budget": 2,
    "critical_after": 0
  }
}
```

### Топ-3 улучшения golden

Запрос:

<!-- example:recommend-improve-request -->
```json
{
  "mode": "improve",
  "decisions": [
    {
      "measure_id": "M7",
      "district_id": "nura"
    },
    {
      "measure_id": "M8",
      "district_id": "nura"
    },
    {
      "measure_id": "M10",
      "district_id": "nura"
    },
    {
      "measure_id": "M12"
    },
    {
      "measure_id": "M5",
      "district_id": "saryarka"
    }
  ]
}
```

Ответ:

<!-- example:recommend-improve-response -->
```json
{
  "current_score": 56.54306999999999,
  "improvements": [
    {
      "decisions": [
        {
          "measure_id": "M10",
          "district_id": "nura"
        },
        {
          "measure_id": "M12"
        },
        {
          "measure_id": "M3",
          "district_id": "nura"
        },
        {
          "measure_id": "M7",
          "district_id": "nura"
        },
        {
          "measure_id": "M8",
          "district_id": "nura"
        }
      ],
      "final_score": 57.20555999999999,
      "total_cost": 100,
      "remaining_budget": 0,
      "critical_after": 0,
      "score_delta": 0.6624899999999982
    },
    {
      "decisions": [
        {
          "measure_id": "M10",
          "district_id": "nura"
        },
        {
          "measure_id": "M12"
        },
        {
          "measure_id": "M14"
        },
        {
          "measure_id": "M7",
          "district_id": "nura"
        },
        {
          "measure_id": "M8",
          "district_id": "nura"
        }
      ],
      "final_score": 56.985820000000004,
      "total_cost": 86,
      "remaining_budget": 14,
      "critical_after": 0,
      "score_delta": 0.44275000000001086
    },
    {
      "decisions": [
        {
          "measure_id": "M10",
          "district_id": "nura"
        },
        {
          "measure_id": "M12"
        },
        {
          "measure_id": "M2"
        },
        {
          "measure_id": "M7",
          "district_id": "nura"
        },
        {
          "measure_id": "M8",
          "district_id": "nura"
        }
      ],
      "final_score": 56.875820000000004,
      "total_cost": 92,
      "remaining_budget": 8,
      "critical_after": 0,
      "score_delta": 0.3327500000000114
    }
  ]
}
```
