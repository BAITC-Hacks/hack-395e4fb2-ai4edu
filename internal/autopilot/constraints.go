package autopilot

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// This guard deliberately recognizes a small set of explicit budget phrases.
// It is not a general natural-language constraint parser. In particular, lower
// bounds ("не меньше", "at least") are not silently converted into upper bounds.
var explicitBudgetPattern = regexp.MustCompile(`(?i)(^|[^\p{L}\p{N}_])(?:бюджет(?:е|ом)?|budget)[\s\p{Zs}]*(?:(?:не[\s\p{Zs}]+(?:больше|более|выше)|до|at[\s\p{Zs}]+most|no[\s\p{Zs}]+more[\s\p{Zs}]+than|not[\s\p{Zs}]+more[\s\p{Zs}]+than|up[\s\p{Zs}]+to|of|<=|≤|:|=)[\s\p{Zs}]*)?([+-]?[0-9]+(?:[.,][0-9]+)?(?:[eE][+-]?[0-9]+)?)`)

var budgetRangePattern = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(?:[-–—/]|или|or|до|to)[\s\p{Zs}]*[+-]?[0-9]`)
var budgetThousandsPattern = regexp.MustCompile(`^[\s\p{Zs}]+[0-9]`)
var budgetMultiplierPattern = regexp.MustCompile(`(?i)^[\s\p{Zs}]*(?:тыс(?:яч)?|млн|миллион|миллиард|thousand|million|billion|k)(?:$|[^\p{L}\p{N}_])`)
var negatedBudgetPattern = regexp.MustCompile(`(?i)(?:^|[^\p{L}\p{N}_])(?:не|not|no)[\s\p{Zs}]*$`)

// explicitBudgetLimit returns an integer cap only for an unambiguous supported
// phrase. Conflicting caps, decimals, ranges and invalid amounts require user
// clarification. Unrecognized wording returns nil, nil, not proof that the user
// provided no constraint. The caller must still retain the original goal.
func explicitBudgetLimit(text string) (*int, error) {
	if len(text) > 16<<10 || !utf8.ValidString(text) {
		return nil, errors.New("budget goal is too long or invalid")
	}
	var limit *int
	for _, match := range explicitBudgetPattern.FindAllStringSubmatchIndex(text, -1) {
		if negatedBudgetPattern.MatchString(text[:match[0]]) {
			return nil, errors.New("negated budget amount requires clarification")
		}
		// Group 1 is the Unicode-aware left word boundary, group 2 the amount.
		start, end := match[4], match[5]
		amount := text[start:end]
		tail := text[end:]
		if len(tail) > 0 {
			r, _ := utf8.DecodeRuneInString(tail)
			if unicode.IsLetter(r) || unicode.IsNumber(r) || r == '_' {
				// Never accept the numeric prefix of another word or notation.
				return nil, errors.New("ambiguous budget amount")
			}
		}
		if budgetRangePattern.MatchString(tail) || budgetThousandsPattern.MatchString(tail) || budgetMultiplierPattern.MatchString(tail) {
			return nil, errors.New("budget needs one integer cap from 1 to 100")
		}
		if strings.ContainsAny(amount, "+-.,eE") {
			return nil, errors.New("budget must be an integer from 1 to 100")
		}
		value, err := strconv.Atoi(amount)
		if err != nil || value < 1 || value > 100 {
			return nil, errors.New("budget must be an integer from 1 to 100")
		}
		if limit != nil && *limit != value {
			return nil, errors.New("multiple different explicit budgets require clarification")
		}
		copy := value
		limit = &copy
	}
	return limit, nil
}
