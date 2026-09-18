package usecases

import (
	"sdd-cli/internal/domain"
	"sdd-cli/internal/ports"
)

type ListAdaptersUseCase struct {
	catalog ports.AdapterCatalog
}

type InstallAdapterUseCase struct {
	installer ports.AdapterInstaller
}

func NewListAdaptersUseCase(catalog ports.AdapterCatalog) *ListAdaptersUseCase {
	return &ListAdaptersUseCase{catalog: catalog}
}

func NewInstallAdapterUseCase(installer ports.AdapterInstaller) *InstallAdapterUseCase {
	return &InstallAdapterUseCase{installer: installer}
}

func (uc *ListAdaptersUseCase) Execute() ([]ports.AdapterDescriptor, error) {
	return uc.catalog.ListAdapters()
}

func (uc *InstallAdapterUseCase) Execute(targetDir, adapterID string) (*ports.AdapterInstallation, error) {
	if err := domain.ValidateIdentifier("adapter id", adapterID); err != nil {
		return nil, err
	}
	return uc.installer.InstallAdapter(targetDir, adapterID)
}
