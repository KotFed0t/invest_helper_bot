package tgCallback

// Callbacks buttons prefixes
const (
	AddStock                   string = "add_stock" // инициировать добавление новой акции
	BackToPortolioFromAddStock string = "back_to_portfolio_from_addstock"
	ChangeWeight               string = "change_weight"
	BuyStock                   string = "buy_stock"
	SaveStockChanges           string = "save_stock_changes"
	AddStockToPortfolio        string = "add_stock_to_portfolio" // добавить конкретный тикер в портфель

	EditStockPrefix string = "edit_stock:"
	PrevPagePrefix  string = "prev_page:"
	NextPagePrefix  string = "next_page:"
)
