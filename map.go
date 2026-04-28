package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/go-gl/mathgl/mgl32"
)

type TileMap struct {
	Width      int        `json:"width"`
	Height     int        `json:"height"`
	TileWidth  int        `json:"tilewidth"`
	TileHeight int        `json:"tileheight"`
	Layers     []MapLayer `json:"layers"`
	Tilesets   []Tileset  `json:"tilesets"`

	Texture *Texture `json:"-"`
}

type MapLayer struct {
	Data    []int  `json:"data"`
	Height  int    `json:"height"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Visible bool   `json:"visible"`
	Width   int    `json:"width"`
}

type Tileset struct {
	Columns     int    `json:"columns"`
	FirstGID    int    `json:"firstgid"`
	Image       string `json:"image"`
	ImageHeight int    `json:"imageheight"`
	ImageWidth  int    `json:"imagewidth"`
	Margin      int    `json:"margin"`
	Name        string `json:"name"`
	Spacing     int    `json:"spacing"`
	TileCount   int    `json:"tilecount"`
	TileHeight  int    `json:"tileheight"`
	TileWidth   int    `json:"tilewidth"`
}

func LoadMap(path string) (*TileMap, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("falha ao ler mapa %q: %w", path, err)
	}

	var tileMap TileMap
	if err := json.Unmarshal(data, &tileMap); err != nil {
		return nil, fmt.Errorf("falha ao decodificar mapa %q: %w", path, err)
	}

	if len(tileMap.Tilesets) == 0 {
		return nil, fmt.Errorf("mapa %q nao possui tileset", path)
	}

	tilesetImagePath := filepath.Join(filepath.Dir(path), tileMap.Tilesets[0].Image)
	texture, err := NewTexture(tilesetImagePath)
	if err != nil {
		return nil, fmt.Errorf("falha ao carregar tileset do mapa: %w", err)
	}
	tileMap.Texture = texture

	return &tileMap, nil
}

func (m *TileMap) WorldWidth() float32 {
	return float32(m.Width * m.TileWidth)
}

func (m *TileMap) WorldHeight() float32 {
	return float32(m.Height * m.TileHeight)
}

//futuramente tem que alterar isso daqui, pq o mapa vai ser bem maior
func (m *TileMap) VisibleRange(cameraPos mgl32.Vec3) (int, int, int, int) {
	worldLeft := cameraPos.X() - windowWidth/2
	worldRight := cameraPos.X() + windowWidth/2
	worldBottom := cameraPos.Y() - windowHeight/2
	worldTop := cameraPos.Y() + windowHeight/2
	worldHeight := m.WorldHeight()

	startCol := int(math.Floor(float64(worldLeft/float32(m.TileWidth)))) - 1
	endCol := int(math.Floor(float64(worldRight/float32(m.TileWidth)))) + 1
	startRow := int(math.Floor(float64((worldHeight-worldTop)/float32(m.TileHeight)))) - 1
	endRow := int(math.Floor(float64((worldHeight-worldBottom)/float32(m.TileHeight)))) + 1

	if startCol < 0 {
		startCol = 0
	}
	if startRow < 0 {
		startRow = 0
	}
	if endCol >= m.Width {
		endCol = m.Width - 1
	}
	if endRow >= m.Height {
		endRow = m.Height - 1
	}

	return startCol, endCol, startRow, endRow
}

func (m *TileMap) TileModel(row, col int) mgl32.Mat4 {
	x := float32(col*m.TileWidth) + float32(m.TileWidth)/2
	y := m.WorldHeight() - float32(row*m.TileHeight) - float32(m.TileHeight)/2

	return mgl32.Translate3D(x, y, 0).
		Mul4(mgl32.Scale3D(float32(m.TileWidth), float32(m.TileHeight), 1))
}

func (m *TileMap) TileUV(gid int) (mgl32.Vec2, mgl32.Vec2, bool) {
	if gid == 0 || len(m.Tilesets) == 0 {
		return mgl32.Vec2{}, mgl32.Vec2{}, false
	}

	tileset := m.Tilesets[0]
	localID := gid - tileset.FirstGID
	if localID < 0 || localID >= tileset.TileCount {
		return mgl32.Vec2{}, mgl32.Vec2{}, false
	}

	col := localID % tileset.Columns
	row := localID / tileset.Columns

	uvScale := mgl32.Vec2{
		float32(tileset.TileWidth) / float32(tileset.ImageWidth),
		float32(tileset.TileHeight) / float32(tileset.ImageHeight),
	}
	uvOffset := mgl32.Vec2{
		float32(col*tileset.TileWidth) / float32(tileset.ImageWidth),
		float32(row*tileset.TileHeight) / float32(tileset.ImageHeight),
	}

	return uvScale, uvOffset, true
}
