package main

import (
	"log"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

const (
	// Largura inicial da janela em pixels.
	windowWidth = 800
	// Altura inicial da janela em pixels.
	windowHeight = 600
	// Titulo mostrado na barra da janela.
	windowTitle = "g-engine"
	// Tamanho do sprite no mundo 2D.
	spriteWidth  = 96
	spriteHeight = 96
	// Velocidade do player em unidades por segundo.
	playerSpeed = 260.0
)

func init() {
	// OpenGL precisa rodar na thread principal.
	runtime.LockOSThread()
}

func main() {
	// Cria a janela e inicializa a GLFW.
	window := initGLFW()
	// Garante que a GLFW sera encerrada ao sair do programa.
	defer glfw.Terminate()

	// Carrega as funcoes OpenGL depois que o contexto estiver ativo.
	initOpenGL()

	// Compila os shaders e cria o programa que a GPU vai usar no draw.
	program, err := NewShaderProgram("assets/shaders/basic.vert", "assets/shaders/basic.frag")
	if err != nil {
		log.Fatalln("erro ao criar shader program:", err)
	}

	// Carrega o tilemap e o tileset atlas usados no cenario.
	tileMap, err := LoadMap("assets/maps/mapa1.tmj")
	if err != nil {
		log.Fatalln("erro ao carregar mapa:", err)
	}

	// Carrega a textura do sprite e faz o upload uma vez para a GPU.
	playerTexture, err := NewTexture("assets/textures/zombie1.png")
	if err != nil {
		log.Fatalln("erro ao carregar textura:", err)
	}

	// O sampler "texture0" vai ler da unidade de textura 0.
	gl.UseProgram(program)
	textureUniform := gl.GetUniformLocation(program, gl.Str("texture0\x00"))
	gl.Uniform1i(textureUniform, 0)
	modelUniform := gl.GetUniformLocation(program, gl.Str("model\x00"))
	viewUniform := gl.GetUniformLocation(program, gl.Str("view\x00"))
	projectionUniform := gl.GetUniformLocation(program, gl.Str("projection\x00"))
	uvScaleUniform := gl.GetUniformLocation(program, gl.Str("uvScale\x00"))
	uvOffsetUniform := gl.GetUniformLocation(program, gl.Str("uvOffset\x00"))

	// Estado inicial do player e da camera.
	playerPos := mgl32.Vec3{tileMap.WorldWidth() / 2, tileMap.WorldHeight() / 2, 0}
	cameraPos := playerPos
	projection := mgl32.Ortho(0, windowWidth, 0, windowHeight, -1, 1)
	lastFrame := glfw.GetTime()

	// Cada vertice tem 5 floats:
	// x, y, z, u, v
	// Os quatro vertices abaixo representam os quatro cantos do quadrado.
	vertices := []float32{
		// posicao         // UV
		-0.5, 0.5, 0.0, 0.0, 0.0,
		0.5, 0.5, 0.0, 1.0, 0.0,
		0.5, -0.5, 0.0, 1.0, 1.0,
		-0.5, -0.5, 0.0, 0.0, 1.0,
	}

	// Dois triangulos formam um quadrado.
	// Esses indices dizem em que ordem os vertices serao usados.
	indices := []uint32{
		0, 1, 2,
		2, 3, 0,
	}

	// VAO: guarda a configuracao dos atributos de vertice.
	var vao uint32 //(Vertex Array Object)
	// VBO: guarda os dados dos vertices na GPU.
	var vbo uint32 //(Vertex Buffer object)
	// EBO: guarda os indices na GPU.
	var ebo uint32 // Element Buffer Object

	// Pede ao OpenGL identificadores para esses objetos.
	gl.GenVertexArrays(1, &vao)
	gl.GenBuffers(1, &vbo)
	gl.GenBuffers(1, &ebo)

	// A partir daqui, a configuracao de atributos ficara vinculada a este VAO.
	gl.BindVertexArray(vao)

	// Envia o array de vertices para a GPU.
	gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	// Envia o array de indices para a GPU.
	gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, ebo)
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices),
		gl.STATIC_DRAW)

	// Define como o atributo 0 deve ser lido:
	// 3 floats por vertice, stride de 5 floats, comecando no byte 0.
	// Isso corresponde ao "layout(location = 0) in vec3 aPos" do shader.
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 5*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	// Define como o atributo 1 deve ser lido:
	// 2 floats por vertice, comecando apos os 3 floats de posicao.
	// Isso corresponde ao "layout(location = 1) in vec2 aTexCoord".
	gl.VertexAttribPointer(1, 2, gl.FLOAT, false, 5*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	// O VBO pode ser desassociado; o VAO ja guardou a configuracao.
	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	// Desassocia o VAO por organizacao.
	gl.BindVertexArray(0)

	// Loop principal da aplicacao.
	for !window.ShouldClose() {
		// Processa eventos de janela, teclado, mouse etc.
		glfw.PollEvents()

		// Calcula o tempo entre frames para manter o movimento suave.
		currentFrame := glfw.GetTime()
		deltaTime := float32(currentFrame - lastFrame)
		lastFrame = currentFrame

		var movement mgl32.Vec3
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

		// Normaliza o vetor para a diagonal nao ser mais rapida.
		if movement.Len() > 0 {
			movement = movement.Normalize()
		}
		playerPos = playerPos.Add(movement.Mul(playerSpeed * deltaTime))
		if movement.Len() > 0 {
			log.Printf(
				"input movimento=(%.2f, %.2f) playerPos=(%.2f, %.2f) deltaTime=%.4f",
				movement.X(),
				movement.Y(),
				playerPos.X(),
				playerPos.Y(),
				deltaTime,
			)
		}

		// Mantem a camera centrada no player.
		cameraPos = playerPos
		model := mgl32.Translate3D(playerPos.X(), playerPos.Y(), 0).
			Mul4(mgl32.Scale3D(spriteWidth, spriteHeight, 1))
		view := mgl32.Translate3D(
			-cameraPos.X()+windowWidth/2,
			-cameraPos.Y()+windowHeight/2,
			0,
		)

		// Define a cor usada para limpar a tela.
		gl.ClearColor(0.08, 0.09, 0.12, 1.0)
		// Limpa o buffer de cor com a cor definida acima.
		gl.Clear(gl.COLOR_BUFFER_BIT)

		// Ativa o programa de shader.
		gl.UseProgram(program)
		gl.UniformMatrix4fv(viewUniform, 1, false, &view[0])
		gl.UniformMatrix4fv(projectionUniform, 1, false, &projection[0])
		gl.BindVertexArray(vao)
		gl.ActiveTexture(gl.TEXTURE0)

		// Renderiza somente os tiles visiveis nas layers do mapa.
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
					gl.UniformMatrix4fv(modelUniform, 1, false, &tileModel[0])
					gl.Uniform2f(uvScaleUniform, uvScale.X(), uvScale.Y())
					gl.Uniform2f(uvOffsetUniform, uvOffset.X(), uvOffset.Y())
					gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, gl.PtrOffset(0))
				}
			}
		}

		// Renderiza o player por cima do mapa usando a textura inteira.
		gl.UniformMatrix4fv(modelUniform, 1, false, &model[0])
		gl.Uniform2f(uvScaleUniform, 1, 1)
		gl.Uniform2f(uvOffsetUniform, 0, 0)
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, playerTexture.ID)
		gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT, gl.PtrOffset(0))

		// Exibe o frame renderizado na janela.
		window.SwapBuffers()
	}
}

func initGLFW() *glfw.Window {
	// Inicializa a biblioteca responsavel por janela, contexto e input.
	if err := glfw.Init(); err != nil {
		log.Fatalln("erro ao inicializar GLFW:", err)
	}

	// A janela nao podera ser redimensionada.
	glfw.WindowHint(glfw.Resizable, glfw.False)
	// Pede um contexto OpenGL 4.1.
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	// Pede o perfil core, sem API legada.
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	// Necessario em alguns sistemas para contexto moderno.
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	// Cria a janela e o contexto OpenGL.
	window, err := glfw.CreateWindow(windowWidth, windowHeight, windowTitle, nil, nil)
	if err != nil {
		log.Fatalln("erro ao criar janela:", err)
	}

	// Torna esse contexto o atual da thread principal.
	window.MakeContextCurrent()
	// Ativa VSync para sincronizar com a taxa de atualizacao do monitor.
	glfw.SwapInterval(1)

	return window
}

func initOpenGL() {
	// Carrega os ponteiros para as funcoes OpenGL do driver atual.
	if err := gl.Init(); err != nil {
		log.Fatalln("erro ao inicializar OpenGL:", err)
	}

	// Imprime a versao do OpenGL detectada.
	log.Println("OpenGL version:", gl.GoStr(gl.GetString(gl.VERSION)))
}
