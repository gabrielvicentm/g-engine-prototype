package main

const (
	windowWidth  = 800
	windowHeight = 600
	windowTitle  = "g-engine"

	spriteWidth  = 96
	spriteHeight = 96
	playerSpeed  = 260.0

	defaultServerAddr = "192.168.0.12:4000"
	serverTickRate    = 20.0

	defaultMapPath    = "assets/maps/mapa1.tmj"
	playerTexturePath = "assets/textures/zombie1.png"

	//TENHO QUE MUDAR ISSO DAQUI PARA ESSES DADOS VIREM DO mapa, não pode ser hardcoded isso, pq se o mapa mudar, tem que alterar isso aqui na mão
	defaultMapTilesWide  = 50
	defaultMapTilesHigh  = 50
	defaultMapTileWidth  = 16
	defaultMapTileHeight = 16
)
