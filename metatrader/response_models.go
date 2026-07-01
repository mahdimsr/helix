package metatrader

type OrderResult struct {
	OK      bool   `json:"ok"`
	Retcode int    `json:"retcode"`
	Ticket  uint64 `json:"ticket"`
	Comment string `json:"comment"`
}
