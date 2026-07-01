package metatrader

import (
	"bufio"
	"fmt"
	"log"
	"net"
)

func Handle(conn net.Conn) {

	defer func(conn net.Conn) {
		err := conn.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(conn)

	r := bufio.NewReader(conn)

	for {
		line, err := r.ReadString('\n')
		if err != nil {
			log.Println("conn closed:", err)
			return
		}

		fmt.Printf("line: %s \n", line)

		fmt.Printf("Set Buy \n")

		err = placeOrder(conn, "BUY")
		if err != nil {
			log.Println("order error:", err)
			return
		}
	}
}

func placeOrder(conn net.Conn, side string) error {

	if _, err := conn.Write([]byte(side + "\n")); err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// خواندن پاسخ JSON
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	fmt.Printf("response form metatrader: %s", line)

	/*var r OrderResult
	if err := json.Unmarshal([]byte(line), &r); err != nil {
		return fmt.Errorf("bad json %q: %w", line, err)
	}*/
	return nil
}
