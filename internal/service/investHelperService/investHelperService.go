package investHelperService

type Cache interface {
}

type Repository interface {
}

type InvestHelperService struct {
	repo  Repository
	cache Cache
}

func New(repo Repository, cache Cache) *InvestHelperService {
	return &InvestHelperService{
		repo: repo,
		cache: cache,
	}
}
