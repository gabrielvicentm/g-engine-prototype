package main

import (
	"fmt"
	"log"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

func RunClient(addr string) error {
	window := initGLFW()
	defer glfw.Terminate()

	initOpenGL()

	program, err := NewShaderProgram("assets/shaders/basic.vert", "assets/shaders/basic.frag")
	if err != nil {
		return fmt.Errorf("erro ao criar shader program: %w", err)
	}

	tileMap, err := LoadMap(defaultMapPath)
	if err != nil {
		return fmt.Errorf("erro ao carregar mapa: %w", err)
	}

	playerTexture, err := NewTexture(playerTexturePath)
	if err != nil {
		return fmt.Errorf("erro ao carregar textura: %w", err)
	}

	gl.UseProgram(program)
	textureUniform := gl.GetUniformLocation(program, gl.Str("texture0\x00"))
	gl.Uniform1i(textureUniform, 0)

	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	viewUniform := gl.GetUniformLocation(program, gl.Str("view\x00"))
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	uvScaleUniform := gl.GetUniformLocation(program, gl.Str("uvScale\x00"))
	uvOffsetUniform := gl.GetUniformLocation(program, gl.Str("uvOffset\x00"))

	vao, indexCount := createQuadMesh()

	world := NewWorld()
	entityByPlayerID := make(map[int]Entity)
	renderer := &RenderContext{
		VAO:               vao,
		IndexCount:        indexCount,
		ModelUniform:      modelUniform,
		ViewUniform:       viewUniform,
		ProjectionUniform: projectionUniform,
		UVScaleUniform:    uvScaleUniform,
		UVOffsetUniform:   uvOffsetUniform,
		Projection:        mgl32.Ortho(0, windowWidth, 0, windowHeight, -1, 1),
	}

	client, err := ConnectToServer(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	log.Println("cliente multiplayer conectado em", addr)

	lastFrame := glfw.GetTime()
	for !window.ShouldClose() {
		glfw.PollEvents()

		currentFrame := glfw.GetTime()
		_ = float32(currentFrame - lastFrame)
		lastFrame = currentFrame

		moveX, moveY := collectNetworkInput(window)
		if err := client.SendInput(moveX, moveY); err != nil {
			return fmt.Errorf("falha ao enviar input ao servidor: %w", err)
		}

		drainWorldUpdates(client, world, entityByPlayerID, playerTexture)

		cameraPos := mgl32.Vec3{tileMap.WorldWidth() / 2, tileMap.WorldHeight() / 2, 0}
		if localEntity, ok := entityByPlayerID[client.PlayerID()]; ok {
			cameraPos = world.Positions[localEntity].Value
		}
		renderer.View = mgl32.Translate3D(
			-cameraPos.X()+windowWidth/2,
			-cameraPos.Y()+windowHeight/2,
			0,
		)

		gl.ClearColor(0.08, 0.09, 0.12, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		gl.UseProgram(program)
		renderTileMap(tileMap, cameraPos, renderer)
		RenderSystem(world, renderer)

		window.SwapBuffers()
	}

	return nil
}

func collectNetworkInput(window *glfw.Window) (float32, float32) {
	var movement mgl32.Vec2
	if window.GetKey(glfw.KeyW) == glfw.Press {
		movement[1] += 1
	}
	if window.GetKey(glfw.KeyS) == glfw.Press {
		movement[1] -= 1
	}
	if window.GetKey(glfw.KeyA) == glfw.Press {
		movement[0] -= 1
	}
	if window.GetKey(glfw.KeyD) == glfw.Press {
		movement[0] += 1
	}
	if movement.Len() > 0 {
		movement = movement.Normalize()
	}
	return movement.X(), movement.Y()
}

func drainWorldUpdates(client *ClientConnection, world *World, entityByPlayerID map[int]Entity, playerTexture *Texture) {
	for {
		select {
		case update, ok := <-client.updates:
			if !ok {
				return
			}
			if update.Type != messageTypeWorldState {
				continue
			}
			syncWorldState(world, entityByPlayerID, update.Players, playerTexture)
		default:
			return
		}
	}
}

func syncWorldState(world *World, entityByPlayerID map[int]Entity, players []NetworkPlayerState, playerTexture *Texture) {
	activePlayers := make(map[int]struct{}, len(players))

	for _, player := range players {
		activePlayers[player.ID] = struct{}{}

		entity, ok := entityByPlayerID[player.ID]
		if !ok {
			entity = world.NewEntity()
			entityByPlayerID[player.ID] = entity
			world.Sprites[entity] = Sprite{
				Texture:  playerTexture,
				Width:    spriteWidth,
				Height:   spriteHeight,
				UVScale:  mgl32.Vec2{1, 1},
				UVOffset: mgl32.Vec2{0, 0},
			}
		}

		world.Positions[entity] = Position{
			Value: mgl32.Vec3{player.X, player.Y, 0},
		}
	}

	for playerID, entity := range entityByPlayerID {
		if _, ok := activePlayers[playerID]; ok {
			continue
		}

		world.DeleteEntity(entity)
		delete(entityByPlayerID, playerID)
	}
}

func renderTileMap(tileMap *TileMap, cameraPos mgl32.Vec3, renderer *RenderContext) {
	gl.UniformMatrix4fv(renderer.ViewUniform, 1, false, &renderer.View[0])
	gl.UniformMatrix4fv(renderer.ProjectionUniform, 1, false, &renderer.Projection[0])
	gl.BindVertexArray(renderer.VAO)
	gl.ActiveTexture(gl.TEXTURE0)

	startCol, endCol, startRow, endRow := tileMap.VisibleRange(cameraPos)
	gl.BindTexture(gl.TEXTURE_2D, tileMap.Texture.ID)
	for _, layer := range tileMap.Layers {
		if !layer.Visible || layer.Type != "tilelayer" {
			continue
		}

		for row := startRow; row <= endRow; row++ {
			for col := startCol; col <= endCol; col++ {
				index := row*layer.Width + col
				gid := layer.Data[index]
				uvScale, uvOffset, ok := tileMap.TileUV(gid)
				if !ok {
					continue
				}

				tileModel := tileMap.TileModel(row, col)
				gl.UniformMatrix4fv(renderer.ModelUniform, 1, false, &tileModel[0])
				gl.Uniform2f(renderer.UVScaleUniform, uvScale.X(), uvScale.Y())
				gl.Uniform2f(renderer.UVOffsetUniform, uvOffset.X(), uvOffset.Y())
				gl.DrawElements(gl.TRIANGLES, renderer.IndexCount, gl.UNSIGNED_INT, gl.PtrOffset(0))
			}
		}
	}
}

func createQuadMesh() (uint32, int32) {
	vertices := []float32{
		-0.5, 0.5, 0.0, 0.0, 0.0,
		0.5, 0.5, 0.0, 1.0, 0.0,
		0.5, -0.5, 0.0, 1.0, 1.0,
		-0.5, -0.5, 0.0, 0.0, 1.0,
	}

	indices := []uint32{
		0, 1, 2,
		2, 3, 0,
	}

	var vao uint32
	var vbo uint32
	var ebo uint32

	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.GenBuffers(1, &ebo)

	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)

	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	return vao, int32(len(indices))
}

func initGLFW() *glfw.Window {
	if err := glfw.Init(); err != nil {
		log.Fatalln("erro ao inicializar GLFW:", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(windowWidth, windowHeight, windowTitle, nil, nil)
	if err != nil {
		log.Fatalln("erro ao criar janela:", err)
	}

	window.MakeContextCurrent()
	glfw.SwapInterval(1)
	return window
}

func initOpenGL() {
	if err := gl.Init(); err != nil {
		log.Fatalln("erro ao inicializar OpenGL:", err)
	}

	log.Println("OpenGL version:", gl.GoStr(gl.GetString(gl.VERSION)))
}
