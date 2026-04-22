package main

import (
	"log"
	"runtime"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

const (
	windowWidth  = 800
	windowHeight = 600
	windowTitle  = "g-engine"
)

func init() {
	// OpenGL precisa rodar na thread principal.
	runtime.LockOSThread()
}

func main() {
	window := initGLFW()
	defer glfw.Terminate()

	initOpenGL()

	program, err := NewShaderProgram("assets/shaders/basic.vert", "assets/shaders/basic.frag")
	if err != nil {
		log.Fatalln("erro ao criar shader program:", err)
	}

	vertices := []float32{
		// posicao        // cor
		-0.5, 0.5, 0.0, 1.0, 0.0, 0.0,
		0.5, 0.5, 0.0, 0.0, 1.0, 0.0,
		0.5, -0.5, 0.0, 0.0, 0.0, 1.0,
		-0.5, -0.5, 0.0, 1.0, 1.0, 0.0,
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
	gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices),
		gl.STATIC_DRAW)

	// layout(location = 0) -> vec3 position
	gl.VertexAttribPointer(0, 3, gl.FLOAT, false, 6*4, gl.PtrOffset(0))
	gl.EnableVertexAttribArray(0)

	// layout(location = 1) -> vec3 color
	gl.VertexAttribPointer(1, 3, gl.FLOAT, false, 6*4, gl.PtrOffset(3*4))
	gl.EnableVertexAttribArray(1)

	gl.BindBuffer(gl.ARRAY_BUFFER, 0)
	gl.BindVertexArray(0)

	for !window.ShouldClose() {
		glfw.PollEvents()

		gl.ClearColor(0.08, 0.09, 0.12, 1.0)
		gl.Clear(gl.COLOR_BUFFER_BIT)

		gl.UseProgram(program)
		gl.BindVertexArray(vao)
		gl.DrawElements(gl.TRIANGLES, int32(len(indices)), gl.UNSIGNED_INT,
			gl.PtrOffset(0))

		window.SwapBuffers()
	}
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
