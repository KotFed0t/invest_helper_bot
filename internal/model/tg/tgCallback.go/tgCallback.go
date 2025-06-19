package tgCallback

// Callback button prefixes
const (
	AddStock                           string = "add_stock" // инициировать добавление новой акции
	BackToPortolio                     string = "back_to_portfolio"
	BackToPortolioList                 string = "back_to_portfolio_list"
	ChangeWeight                       string = "change_weight"
	BuyStock                           string = "buy_stock"
	SellStock                          string = "sell_stock"
	DeleteStock                        string = "delete_stock"
	ChangePrice                        string = "change_price"
	SaveStockChanges                   string = "save_stock_changes"
	AddStockToPortfolio                string = "add_stock_to_portfolio" // добавить конкретный тикер в портфель
	PageNumber                         string = "page_number"
	CalculatePurchase                  string = "calculate_purchase"
	RebalanceWeights                   string = "rebalance_weights"
	InitDeletePortfolio                string = "init_delete_portfolio"
	ProcessDeletePortfolio             string = "process_delete_portfolio"
	GenerateReport                     string = "generate_report"
	ApplyCalculatedPurchaseToPortfolio string = "apply_calculated_purchase_to_portolio"
	CreatePortfolio                    string = "create_portolio"

	// prefixes
	EditStockPrefix     string = "edit_stock:"
	ToPortfolioPage     string = "to_portfolio_page:"
	EditPortfolioPrefix string = "edit_portfolio:"
	ToPortfolioListPage string = "to_portfolio_list_page:"
)
