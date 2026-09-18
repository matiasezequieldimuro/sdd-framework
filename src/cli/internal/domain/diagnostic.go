package domain

import "fmt"

type CheckStatus string

const (
	CheckPassed  CheckStatus = "passed"
	CheckWarning CheckStatus = "warning"
	CheckFailed  CheckStatus = "failed"
)

type ValidationCheck struct {
	Status   CheckStatus `json:"status"`
	Category string      `json:"category"`
	Code     string      `json:"code"`
	Target   string      `json:"target"`
	Message  string      `json:"message"`
}

type ContractViolation struct {
	Code    string
	Message string
	Cause   error
}

func (violation ContractViolation) Error() string {
	if violation.Cause == nil {
		return violation.Message
	}
	return fmt.Sprintf("%v: %s", violation.Cause, violation.Message)
}

func (violation ContractViolation) Unwrap() error {
	return violation.Cause
}

func NewContractViolation(code string, cause error, format string, args ...interface{}) ContractViolation {
	return ContractViolation{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Cause:   cause,
	}
}
