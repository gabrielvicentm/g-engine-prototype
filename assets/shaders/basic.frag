#version 410 core

// Coordenada UV interpolada recebida do vertex shader.
in vec2 vTexCoord;

// Cor final do pixel atual.
out vec4 FragColor;

// Textura associada ao sprite.
uniform sampler2D texture0;

void main() {
    // Amostra a textura na coordenada UV interpolada.
    FragColor = texture(texture0, vTexCoord);
}
