package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
)

func NewShaderProgram(vertexPath, fragmentPath string) (uint32, error) {
	// Le o codigo-fonte do vertex shader do disco.
	vertexSource, err := os.ReadFile(vertexPath)
	if err != nil {
		return 0, fmt.Errorf("falha ao ler vertex shader: %w", err)
	}

	// Le o codigo-fonte do fragment shader do disco.
	fragmentSource, err := os.ReadFile(fragmentPath)
	if err != nil {
		return 0, fmt.Errorf("falha ao ler fragment shader: %w", err)
	}

	// Compila o vertex shader.
	// O "\x00" e o terminador nulo esperado pela API C do OpenGL.
	vertexShader, err := compileShader(string(vertexSource)+"\x00", gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	// Compila o fragment shader.
	fragmentShader, err := compileShader(string(fragmentSource)+"\x00", gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	// Cria o programa que vai combinar os dois shaders.
	program := gl.CreateProgram()
	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)

	// Linka o programa: aqui o driver valida se os shaders combinam entre si.
	gl.LinkProgram(program)

	// Confere se o link foi bem-sucedido.
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		// Recupera a mensagem de erro retornada pelo driver.
		logMsg := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(logMsg))

		return 0, fmt.Errorf("erro ao linkar programa: %s", logMsg)
	}

	// Depois do link, os shaders individuais ja podem ser descartados.
	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	// Cria um objeto de shader vazio no driver.
	shader := gl.CreateShader(shaderType)

	// Converte a string Go para o formato esperado pelo OpenGL.
	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)

	// Libera a memoria temporaria criada por gl.Strs.
	free()

	// Pede ao driver para compilar o GLSL.
	gl.CompileShader(shader)

	// Verifica se a compilacao funcionou.
	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		// Recupera a mensagem de erro da compilacao.
		logMsg := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(logMsg))

		return 0, fmt.Errorf("erro ao compilar shader: %s", logMsg)
	}

	return shader, nil
}
