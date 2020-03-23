package main

import "github.com/hypebeast/go-osc/osc"

func main() {
	addr := "0.0.0.0:8765"
	d := osc.NewStandardDispatcher()
	d.AddMsgHandler("*", func(msg *osc.Message) {
		osc.PrintMessage(msg)
	})

	server := &osc.Server{
		Addr:       addr,
		Dispatcher: d,
	}
	server.ListenAndServe()
}
