#version 410 core

// Entrada 0: posicao do vertice vinda do VBO.
layout (location = 0) in vec3 aPos;

// Entrada 1: coordenada UV da textura vinda do VBO.
layout (location = 1) in vec2 aTexCoord;

// Matrizes usadas para posicionar o sprite no mundo e projetar na tela.
uniform mat4 model;
uniform mat4 view;
uniform mat4 projection;

// Coordenada de textura interpolada para o fragment shader.
out vec2 vTexCoord;

void main() {
    // Converte o vertice do espaco local para a tela.
    gl_Position = projection * view * model * vec4(aPos, 1.0);

    // Repassa a UV do vertice para o proximo estagio do pipeline.
    vTexCoord = aTexCoord;
}
