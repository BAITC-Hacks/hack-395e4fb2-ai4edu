export type Metric = 'T1'|'T2'|'E1'|'E2'|'S1'|'S2'|'B1'|'B2'|'C1'|'C2';
export const categories = ['Transport', 'Ecology', 'Social', 'Safety', 'Services'] as const;
export type Category = typeof categories[number];
export type Indicators = Record<Metric, number>;
export type Effects = Partial<Indicators>;
export interface District { id: string; name: string; population_share: number; indicators: Indicators }
export interface Measure { id: string; name: string; category: Category; scope: 'City'|'District'; cost: number; lag: number; effects: Effects }
export interface Decision { measure_id: string; district_id?: string }
export interface MeasureFocus { districtId: string; metric: Metric }
export interface Synergy { measure_ids: [string, string]; district_measure_id: string; effects: Effects }
export interface Scenario {
  budget: number; required_decisions: number; max_measures_per_category: number; horizon_quarters: number;
  indicator_order: Metric[]; indicator_names: Record<Metric, string>; category_names: Record<Category, string>;
  indicator_weights: Indicators;
  scoring: { average_weight: number; minimum_weight: number; critical_threshold: number; critical_penalty: number };
  districts: District[]; measures: Measure[]; synergies: Synergy[];
  incompatibilities: { measure_ids: [string, string]; same_district_only: boolean }[];
}
export interface ValidationError { code: string; message: string; measure_ids?: string[] }
export interface PlanAssessment { valid: boolean; total_cost: number; remaining_budget: number; decisions: Decision[]; validation_errors?: ValidationError[]; critical_after?:number }
export interface Candidate { decisions: Decision[]; final_score: number; total_cost: number; remaining_budget: number; critical_after: number }
export interface Improvement extends Candidate { score_delta:number }
export interface ImproveResponse { current_score:number; improvements:Improvement[] }
export interface DistrictResult {
  district_id: string; name: string; population_share: number; before: Indicators; after: Indicators;
  score_before: number; score_after: number;
}
export interface AppliedEffect { measure_id: string; district_id: string; lag_factor: number; effects: Effects }
export interface AppliedSynergy { measure_ids: [string, string]; district_id: string; effects: Effects }
export interface ScoreBreakdown { weighted_average: number; minimum_district: number; critical_penalty: number }
export interface SimulationResult {
  valid: true; total_cost: number; remaining_budget: number; decisions: Decision[];
  base_score: number; final_score: number; score_delta: number; critical_before: number; critical_after: number;
  district_before_after: DistrictResult[]; indicator_deltas: Record<string, Indicators>;
  applied_effects: AppliedEffect[]; applied_synergies: AppliedSynergy[];
  base_breakdown: ScoreBreakdown; final_breakdown: ScoreBreakdown;
}
export interface Explanation { source: 'llm'|'fallback'; text: string }
export interface ExplainResponse { result: SimulationResult; explanation: Explanation }
export interface SavedRun {
  id: string; name: string; createdAt: string; scenario: Scenario; datasetKey: string;
  result: SimulationResult; explanation?: Explanation;
}
export type Page = 'overview'|'decisions'|'result'|'history';
