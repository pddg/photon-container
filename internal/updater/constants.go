package updater

type UpdateStrategy string

const (
	UpdateStrategySequential UpdateStrategy = "sequential"
	UpdateStrategyParallel   UpdateStrategy = "parallel"

	DefaultUpdateStrategy = UpdateStrategySequential
)

func NewUpdateStrategy(strategy string) UpdateStrategy {
	switch strategy {
	case string(UpdateStrategySequential):
		return UpdateStrategySequential
	case string(UpdateStrategyParallel):
		return UpdateStrategyParallel
	default:
		return UpdateStrategySequential
	}
}
