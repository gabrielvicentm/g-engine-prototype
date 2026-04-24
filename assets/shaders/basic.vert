#version 410 core

// Entrada 0: posicao do vertice vinda do VBO.
layout (location = 0) in vec3 aPos;

// Entrada 1: coordenada UV da textura vinda do VBO.
layout (location = 1) in vec2 aTexCoord;

// Coordenada de textura interpolada para o fragment shader.
out vec2 vTexCoord;

void main() {
    // Define a posicao final do vertice no espaco de clip.
    gl_Position = vec4(aPos, 1.0);

    // Repassa a UV do vertice para o proximo estagio do pipeline.
    vTexCoord = aTexCoord;
}
