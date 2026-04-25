package main

import (
	"flag"
	"log"
	"runtime"
)

func init() {
	// OpenGL precisa rodar na thread principal.
	runtime.LockOSThread()
}

func main() {
	mode := flag.String("mode", "client", "modo de execucao: client ou server")
	addr := flag.String("addr", defaultServerAddr, "endereco TCP do servidor")
	flag.Parse()

	switch *mode {
	case "server":
		if err := RunServer(*addr); err != nil {
			log.Fatalln("erro ao executar servidor:", err)
		}
	case "client":
		if err := RunClient(*addr); err != nil {
			log.Fatalln("erro ao executar cliente:", err)
		}
	default:
		log.Fatalf("modo invalido %q, use client ou server", *mode)
	}
}
