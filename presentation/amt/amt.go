package amt

import amt "github.com/Gleb988/online-shop_amt"

// главное здесь отправить этот тип ошибки, когда понятно, что повторные попытки тоже приведут к провалу
func zxc() {
	amt.NewErrNack("123")
}
