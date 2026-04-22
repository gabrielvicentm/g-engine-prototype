 package main

  import (
  	"fmt"
  	"os"
  	"strings"

  	"github.com/go-gl/gl/v4.1-core/gl"
  )

  func NewShaderProgram(vertexPath, fragmentPath string) (uint32, error) {
  	vertexSource, err := os.ReadFile(vertexPath)
  	if err != nil {
  		return 0, fmt.Errorf("falha ao ler vertex shader: %w", err)
  	}

  	fragmentSource, err := os.ReadFile(fragmentPath)
  	if err != nil {
  		return 0, fmt.Errorf("falha ao ler fragment shader: %w", err)
  	}

  	vertexShader, err := compileShader(string(vertexSource)+"\x00", gl.VERTEX_SHADER)
  	if err != nil {
  		return 0, err
  	}

  	fragmentShader, err := compileShader(string(fragmentSource)+"\x00",
  gl.FRAGMENT_SHADER)
  	if err != nil {
  		return 0, err
  	}

  	program := gl.CreateProgram()
  	gl.AttachShader(program, vertexShader)
  	gl.AttachShader(program, fragmentShader)
  	gl.LinkProgram(program)

  	var status int32
  	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
  	if status == gl.FALSE {
  		var logLength int32
  		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

  		logMsg := strings.Repeat("\x00", int(logLength+1))
  		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(logMsg))

  		return 0, fmt.Errorf("erro ao linkar programa: %s", logMsg)
  	}

  	gl.DeleteShader(vertexShader)
  	gl.DeleteShader(fragmentShader)

  	return program, nil
  }

  func compileShader(source string, shaderType uint32) (uint32, error) {
  	shader := gl.CreateShader(shaderType)

  	csources, free := gl.Strs(source)
  	gl.ShaderSource(shader, 1, csources, nil)
  	free()

  	gl.CompileShader(shader)

  	var status int32
  	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
  	if status == gl.FALSE {
  		var logLength int32
  		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

  		logMsg := strings.Repeat("\x00", int(logLength+1))
  		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(logMsg))

  		return 0, fmt.Errorf("erro ao compilar shader: %s", logMsg)
  	}

  	return shader, nil
  }