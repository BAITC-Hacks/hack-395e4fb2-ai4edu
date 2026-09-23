package autopilot

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"hack-395e4fb2-ai4edu/internal/optimizer"
	"hack-395e4fb2-ai4edu/internal/simulation"
)

var (
	ErrDisabled = errors.New("autopilot is not configured")
	ErrBusy     = errors.New("autopilot is busy")
	ErrGoal     = errors.New("goal must be at most 2000 characters")
)

type SearchFunc func(context.Context, optimizer.Goal, int) (optimizer.SearchResult, error)
type Config struct {
	MaxIterations int
	RunBudgetUSD  float64
	Timeout       time.Duration
	Retention     time.Duration
	MaxRuns       int
	Concurrency   int
	Search        SearchFunc
}

type job struct {
	run    Run
	cancel context.CancelFunc
	done   bool
}
type Service struct {
	mu     sync.Mutex
	agents Agents
	budget *Budget
	config Config
	jobs   map[string]*job
	slots  chan struct{}
}

func New(agents Agents, budget *Budget, config Config) *Service {
	if config.MaxIterations <= 0 || config.MaxIterations > 5 {
		config.MaxIterations = 3
	}
	if config.RunBudgetUSD <= 0 {
		config.RunBudgetUSD = .10
	}
	if config.Timeout <= 0 {
		config.Timeout = 3 * time.Minute
	}
	if config.Retention <= 0 {
		config.Retention = time.Hour
	}
	if config.MaxRuns <= 0 {
		config.MaxRuns = 100
	}
	if config.Concurrency <= 0 {
		config.Concurrency = 2
	}
	if config.Search == nil {
		config.Search = optimizer.Search
	}
	return &Service{agents: agents, budget: budget, config: config, jobs: map[string]*job{}, slots: make(chan struct{}, config.Concurrency)}
}

func (s *Service) Start(request Request) (Run, error) {
	if s == nil || s.agents == nil || s.budget == nil {
		return Run{}, ErrDisabled
	}
	if !utf8.ValidString(request.Goal) || utf8.RuneCountInString(request.Goal) > 2000 {
		return Run{}, ErrGoal
	}
	select {
	case s.slots <- struct{}{}:
	default:
		return Run{}, ErrBusy
	}
	s.mu.Lock()
	now := time.Now().UTC()
	for id, j := range s.jobs {
		if j.done && now.Sub(j.run.UpdatedAt) > s.config.Retention {
			delete(s.jobs, id)
		}
	}
	if len(s.jobs) >= s.config.MaxRuns {
		var oldest string
		var timestamp time.Time
		for id, j := range s.jobs {
			if j.done && (oldest == "" || j.run.UpdatedAt.Before(timestamp)) {
				oldest = id
				timestamp = j.run.UpdatedAt
			}
		}
		if oldest == "" {
			s.mu.Unlock()
			<-s.slots
			return Run{}, ErrBusy
		}
		delete(s.jobs, oldest)
	}
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		s.mu.Unlock()
		<-s.slots
		return Run{}, errors.New("cannot create run identifier")
	}
	id := hex.EncodeToString(random[:])
	ctx, cancel := context.WithTimeout(context.Background(), s.config.Timeout)
	run := Run{ID: id, Status: "queued", Goal: strings.TrimSpace(request.Goal), Events: []Event{}, Usage: []Usage{}, BudgetUSD: s.config.RunBudgetUSD, CreatedAt: now, UpdatedAt: now, Message: "Задача принята мастер-агентом."}
	s.jobs[id] = &job{run: run, cancel: cancel}
	s.mu.Unlock()
	go func() { defer func() { cancel(); <-s.slots }(); s.execute(ctx, id) }()
	return run, nil
}

func (s *Service) Get(id string) (Run, bool) {
	if s == nil {
		return Run{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Run{}, false
	}
	if j.done && time.Since(j.run.UpdatedAt) > s.config.Retention {
		delete(s.jobs, id)
		return Run{}, false
	}
	return clone(j.run), true
}

func (s *Service) Cancel(id string) (Run, bool) {
	if s == nil {
		return Run{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[id]
	if !ok {
		return Run{}, false
	}
	if !j.done {
		j.cancel()
		j.done = true
		j.run.Status = "cancelled"
		j.run.Message = "Запуск остановлен. Уже выполненные API-запросы могут быть оплачены."
		j.run.UpdatedAt = time.Now().UTC()
		j.run.Events = append(j.run.Events, Event{Agent: "master", Stage: "cancelled", Message: j.run.Message, Iteration: j.run.Iterations, At: j.run.UpdatedAt})
	}
	return clone(j.run), true
}

// Close stops pending work during server shutdown; it never starts new requests.
func (s *Service) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if !j.done {
			j.cancel()
		}
	}
}

func clone(run Run) Run {
	data, _ := json.Marshal(run)
	var out Run
	_ = json.Unmarshal(data, &out)
	return out
}
func (s *Service) update(id string, fn func(*Run)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil && !j.done {
		fn(&j.run)
		j.run.UpdatedAt = time.Now().UTC()
	}
}
func (s *Service) event(id, agent, stage, message string, iteration int) {
	s.update(id, func(r *Run) {
		r.Events = append(r.Events, Event{agent, stage, message, iteration, time.Now().UTC()})
		r.Message = message
	})
}
func (s *Service) finish(id, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if j := s.jobs[id]; j != nil && !j.done {
		j.done = true
		j.run.Status = status
		j.run.Message = message
		j.run.UpdatedAt = time.Now().UTC()
		j.run.Events = append(j.run.Events, Event{"master", status, message, j.run.Iterations, j.run.UpdatedAt})
	}
}

func (s *Service) call(ctx context.Context, id, role string, input any, output any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return errors.New("cannot encode agent input")
	}
	reserve, err := s.agents.Estimate(role, data)
	if err != nil {
		return err
	}
	run, _ := s.Get(id)
	if run.EstimatedCostUSD+reserve > run.BudgetUSD {
		return ErrBudget
	}
	if err = s.budget.reserve(reserve); err != nil {
		return err
	}
	usage, callErr := s.agents.Call(ctx, role, data, output)
	// An interrupted or failed request may still be billed. Unknown usage is
	// charged at the full reservation, including after cancellation.
	if usage.Uncertain || (callErr != nil && usage.InputTokens == 0 && usage.OutputTokens == 0) {
		usage.Uncertain = true
		usage.EstimatedCostUSD = reserve
	}
	settleErr := s.budget.settle(reserve, usage.EstimatedCostUSD)
	if settleErr != nil {
		usage.Uncertain = true
		usage.EstimatedCostUSD = reserve
	}
	s.mu.Lock()
	if j := s.jobs[id]; j != nil {
		j.run.Usage = append(j.run.Usage, usage)
		j.run.EstimatedCostUSD += usage.EstimatedCostUSD
	}
	s.mu.Unlock()
	if settleErr != nil {
		return settleErr
	}
	return callErr
}

func (s *Service) fail(ctx context.Context, id string, err error) {
	if errors.Is(err, ErrBudget) {
		s.finish(id, "budget_exceeded", "Достигнут лимит API-расходов. Проверенный промежуточный план сохранён, если он найден.")
		return
	}
	if ctx.Err() == context.DeadlineExceeded {
		s.finish(id, "timeout", "Истекло время запуска. Задача не отмечена выполненной.")
		return
	}
	if ctx.Err() != nil {
		s.finish(id, "cancelled", "Запуск остановлен.")
		return
	}
	s.finish(id, "unavailable", "Не удалось завершить AI-проверку. Проверенный промежуточный результат сохранён, если он найден; можно повторить запуск.")
}

func (s *Service) execute(ctx context.Context, id string) {
	defer func() {
		if recover() != nil {
			s.finish(id, "failed", "Запуск завершён из-за внутренней ошибки. Повторите позже.")
		}
	}()
	s.update(id, func(r *Run) { r.Status = "running" })
	run, _ := s.Get(id)
	s.event(id, "master", "interpreting", "Мастер переводит цель в ограничения для поиска.", 0)
	var brief Brief
	if err := s.call(ctx, id, "master", map[string]any{"user_goal": run.Goal, "scenario": simulation.DefaultScenario()}, &brief); err != nil {
		s.fail(ctx, id, err)
		return
	}
	s.update(id, func(r *Run) { r.Brief = &brief })
	if strings.TrimSpace(brief.Clarification) != "" {
		s.finish(id, "needs_clarification", brief.Clarification)
		return
	}
	if err := optimizer.ValidateGoal(brief.Goal); err != nil {
		s.finish(id, "needs_clarification", "Не удалось однозначно перевести цель в поддерживаемые ограничения. Укажите бюджет, приоритетный район или цель повышения Score.")
		return
	}
	s.event(id, "master", "delegating", "Цель зафиксирована. Мастер поручил поиск планировщику и проверку рецензенту.", 0)
	s.event(id, "planner", "searching", "Планировщик вызывает полный поиск допустимых планов в Go.", 0)
	search, err := s.config.Search(ctx, brief.Goal, 5)
	s.update(id, func(r *Run) {
		r.Evaluated = search.Evaluated
		r.Feasible = search.Feasible
		r.Exhaustive = search.Exhaustive
	})
	if err != nil {
		s.fail(ctx, id, err)
		return
	}
	if !search.Exhaustive {
		s.finish(id, "failed", "Поиск не завершён; оптимальность и невыполнимость не подтверждены.")
		return
	}
	if len(search.Candidates) == 0 {
		s.finish(id, "infeasible", "Полный перебор не нашёл план из пяти мер, удовлетворяющий всем ограничениям. Измените бюджет или требования.")
		return
	}
	// Recalculate tool results at the trust boundary, even for injected search
	// implementations. Models only select a server-owned candidate index.
	candidates := make([]simulation.Result, 0, len(search.Candidates))
	for _, candidate := range search.Candidates {
		verified := simulation.Simulate(candidate.Decisions)
		if !optimizer.SatisfiesGoal(brief.Goal, verified) {
			s.finish(id, "failed", "Результат поиска не прошёл повторную проверку ограничений.")
			return
		}
		candidates = append(candidates, verified)
	}
	s.update(id, func(r *Run) { r.Result = &candidates[0] })
	// Resolve each candidate separately: a shared measure catalogue alone makes
	// it too easy for the writer to mix actions from different alternatives.
	candidateActions := make([][]resolvedAction, len(candidates))
	for i, candidate := range candidates {
		candidateActions[i] = resolveActions(candidate)
	}
	s.event(id, "planner", "searched", fmt.Sprintf("Проверено %d сценариев; ограничениям соответствуют %d. Подготовлены %d лучших вариантов.", search.Evaluated, search.Feasible, len(candidates)), 0)
	var previous *Review
	lastProposal := ""
	for iteration := 1; iteration <= s.config.MaxIterations; iteration++ {
		if err = ctx.Err(); err != nil {
			s.fail(ctx, id, err)
			return
		}
		s.update(id, func(r *Run) { r.Iterations = iteration })
		s.event(id, "planner", "proposing", "Планировщик выбирает проверенный вариант и готовит обоснование.", iteration)
		var proposal Proposal
		input := map[string]any{"user_goal": run.Goal, "brief": brief, "candidates": candidates, "candidate_actions": candidateActions, "scenario_context": scenarioContext(candidates), "previous_review": previous, "iteration": iteration}
		if err = s.call(ctx, id, "planner", input, &proposal); err != nil {
			s.fail(ctx, id, err)
			return
		}
		if proposal.CandidateIndex < 0 || proposal.CandidateIndex >= len(candidates) || strings.TrimSpace(proposal.Explanation) == "" {
			previous = &Review{Feedback: "Выбери существующий candidate_index из списка и дай непустое обоснование по проверенным данным.", Tradeoffs: []string{}}
			s.event(id, "master", "revision", "Мастер отклонил неполное предложение и вернул его планировщику.", iteration)
			continue
		}
		if !optimizer.EquallyRanked(brief.Goal, candidates[proposal.CandidateIndex], candidates[0]) {
			previous = &Review{Feedback: "Выбранный вариант хуже лучшего по зафиксированной цели. Выбери candidate_index=0 или вариант с тем же значением цели и итоговым Score; ограничения и цель менять нельзя.", Tradeoffs: []string{}}
			s.event(id, "master", "revision", "Мастер отклонил вариант, уступающий проверенному оптимуму по заданной цели.", iteration)
			continue
		}
		fingerprint := fmt.Sprintf("%d:%s", proposal.CandidateIndex, strings.TrimSpace(proposal.Explanation))
		if previous != nil && fingerprint == lastProposal {
			s.finish(id, "needs_review", "Планировщик повторил отклонённый ответ без изменений. Сохранён лучший проверенный план; уточните цель или повторите позже.")
			return
		}
		lastProposal = fingerprint
		selected := candidates[proposal.CandidateIndex]
		s.update(id, func(r *Run) { r.Result = &selected; r.Explanation = proposal.Explanation; r.Review = nil })
		s.event(id, "reviewer", "checking", "Рецензент проверяет соответствие цели, объяснение и потери по районам.", iteration)
		var review Review
		if err = s.call(ctx, id, "reviewer", map[string]any{"user_goal": run.Goal, "brief": brief, "result": selected, "selected_actions": candidateActions[proposal.CandidateIndex], "best_result": candidates[0], "scenario_context": scenarioContext([]simulation.Result{selected}), "proposal": proposal, "previous_review": previous}, &review); err != nil {
			s.fail(ctx, id, err)
			return
		}
		if review.Tradeoffs == nil {
			review.Tradeoffs = []string{}
		}
		if err = ctx.Err(); err != nil {
			s.fail(ctx, id, err)
			return
		}
		s.update(id, func(r *Run) { r.Review = &review })
		if review.Approved && strings.TrimSpace(review.Feedback) != "" && optimizer.SatisfiesGoal(brief.Goal, selected) {
			s.event(id, "master", "verified", "Мастер получил одобрение рецензента и подтвердил расчёт и ограничения в Go.", iteration)
			s.finish(id, "completed", "Готовый план рассчитан и проверен. Его можно использовать в симуляторе.")
			return
		}
		previous = &review
		s.event(id, "master", "revision", "Рецензент нашёл замечания. Мастер отправил их планировщику для исправления.", iteration)
	}
	s.finish(id, "needs_review", "Лимит итераций достигнут. План рассчитан, но AI-проверка не завершена; задача не отмечена выполненной.")
}

type resolvedAction struct {
	MeasureID string `json:"measure_id"`
	Name      string `json:"name"`
	Location  string `json:"location"`
}

func resolveActions(result simulation.Result) []resolvedAction {
	s := simulation.DefaultScenario()
	names, districts := map[string]string{}, map[string]string{}
	for _, measure := range s.Measures {
		names[measure.ID] = measure.Name
	}
	for _, district := range s.Districts {
		districts[district.ID] = district.Name
	}
	actions := make([]resolvedAction, 0, len(result.Decisions))
	for _, decision := range result.Decisions {
		location := "Весь город"
		if decision.DistrictID != nil {
			location = districts[*decision.DistrictID]
		}
		actions = append(actions, resolvedAction{MeasureID: decision.MeasureID, Name: names[decision.MeasureID], Location: location})
	}
	return actions
}

func scenarioContext(results []simulation.Result) any {
	s := simulation.DefaultScenario()
	ids := map[string]bool{}
	for _, result := range results {
		for _, d := range result.Decisions {
			ids[d.MeasureID] = true
		}
	}
	measures := []simulation.Measure{}
	for _, measure := range s.Measures {
		if ids[measure.ID] {
			measures = append(measures, measure)
		}
	}
	return map[string]any{"measures": measures, "indicator_names": s.IndicatorNames, "category_names": s.CategoryNames, "horizon_quarters": s.Horizon, "scoring": s.Scoring}
}
