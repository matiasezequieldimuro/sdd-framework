package usecases

import "sdd-cli/internal/ports"

type InitUseCase struct {
	initializer ports.ProjectInitializer
}

func NewInitUseCase(initializer ports.ProjectInitializer) *InitUseCase {
	return &InitUseCase{initializer: initializer}
}

func (uc *InitUseCase) Execute(targetDir string) error {
	return uc.initializer.Initialize(targetDir)
}
