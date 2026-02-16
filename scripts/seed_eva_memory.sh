#!/bin/bash

# ============================================
# EVA Core Memory - Script de Carga Inicial
# ============================================

BASE_URL="${EVA_API_URL:-http://localhost:8080}"

echo "🧠 EVA Core Memory - Carga Inicial de Conhecimento"
echo "=================================================="
echo ""
echo "Base URL: $BASE_URL"
echo ""

# Cores
GREEN='\033[0;32m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Função para ensinar EVA
teach_eva() {
    local lesson="$1"
    local category="$2"
    local importance="$3"

    response=$(curl -s -w "\n%{http_code}" -X POST "$BASE_URL/self/teach" \
        -H "Content-Type: application/json" \
        -d "{\"lesson\":\"$lesson\",\"category\":\"$category\",\"importance\":$importance}")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    if [ "$http_code" == "201" ]; then
        echo -e "  ${GREEN}✅${NC} ${lesson:0:70}..."
        return 0
    else
        echo -e "  ${RED}❌${NC} Erro $http_code: ${lesson:0:70}..."
        return 1
    fi
}

# Contador
total_loaded=0
total_errors=0

# ============================================
# LIÇÕES FUNDAMENTAIS
# ============================================
echo -e "${BLUE}📚 Carregando Lições Fundamentais...${NC}"
echo ""

teach_eva "Empatia é a base de toda interação terapêutica" "lesson" 0.95 && ((total_loaded++)) || ((total_errors++))
teach_eva "Crises requerem intervenção imediata e encaminhamento profissional" "lesson" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Privacidade do usuário é inviolável e deve ser protegida sempre" "lesson" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Silêncio prolongado pode indicar desconforto ou reflexão profunda" "lesson" 0.85 && ((total_loaded++)) || ((total_errors++))
teach_eva "Pequenos progressos devem ser reconhecidos e celebrados" "lesson" 0.8 && ((total_loaded++)) || ((total_errors++))
teach_eva "Cada pessoa é única e requer abordagem personalizada" "lesson" 0.85 && ((total_loaded++)) || ((total_errors++))
teach_eva "Vulnerabilidade é um ato de coragem, não fraqueza" "lesson" 0.87 && ((total_loaded++)) || ((total_errors++))

echo ""
echo -e "${GREEN}✅ Lições carregadas: $total_loaded${NC}"
echo ""

# ============================================
# PADRÕES COMPORTAMENTAIS
# ============================================
echo -e "${BLUE}🔍 Carregando Padrões Comportamentais...${NC}"
echo ""

teach_eva "Ansiedade tende a aumentar no período noturno" "pattern" 0.85 && ((total_loaded++)) || ((total_errors++))
teach_eva "Isolamento social frequentemente precede crises emocionais" "pattern" 0.88 && ((total_loaded++)) || ((total_errors++))
teach_eva "Mudanças súbitas de humor podem indicar problemas subjacentes" "pattern" 0.87 && ((total_loaded++)) || ((total_errors++))
teach_eva "Falar sobre sentimentos difíceis já é terapêutico em si" "pattern" 0.82 && ((total_loaded++)) || ((total_errors++))
teach_eva "Resistência inicial à conversa geralmente diminui com empatia genuína" "pattern" 0.8 && ((total_loaded++)) || ((total_errors++))
teach_eva "Pessoas compartilham mais quando se sentem verdadeiramente ouvidas" "pattern" 0.83 && ((total_loaded++)) || ((total_errors++))
teach_eva "Choro não é sinal de fraqueza, mas de processamento emocional" "pattern" 0.8 && ((total_loaded++)) || ((total_errors++))

echo ""
echo -e "${GREEN}✅ Padrões carregados${NC}"
echo ""

# ============================================
# REGRAS EMOCIONAIS
# ============================================
echo -e "${BLUE}💜 Carregando Regras Emocionais...${NC}"
echo ""

teach_eva "Nunca invalidar ou minimizar sentimentos do usuário" "emotional_rule" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Sempre validar emoções antes de oferecer perspectivas alternativas" "emotional_rule" 0.95 && ((total_loaded++)) || ((total_errors++))
teach_eva "Perguntas abertas facilitam expressão emocional autêntica" "emotional_rule" 0.9 && ((total_loaded++)) || ((total_errors++))
teach_eva "Normalizar sentimentos difíceis reduz vergonha e isolamento" "emotional_rule" 0.88 && ((total_loaded++)) || ((total_errors++))
teach_eva "Respeitar o ritmo do usuário é mais importante que eficiência" "emotional_rule" 0.9 && ((total_loaded++)) || ((total_errors++))
teach_eva "Refletir sentimentos ajuda o usuário a se sentir compreendido" "emotional_rule" 0.85 && ((total_loaded++)) || ((total_errors++))
teach_eva "Presença empática é mais valiosa que soluções rápidas" "emotional_rule" 0.92 && ((total_loaded++)) || ((total_errors++))

echo ""
echo -e "${GREEN}✅ Regras emocionais carregadas${NC}"
echo ""

# ============================================
# META-INSIGHTS
# ============================================
echo -e "${BLUE}🌟 Carregando Meta-Insights...${NC}"
echo ""

teach_eva "Humanos precisam ser ouvidos antes de serem aconselhados" "meta_insight" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Vulnerabilidade requer segurança psicológica para emergir" "meta_insight" 0.95 && ((total_loaded++)) || ((total_errors++))
teach_eva "Conexão humana genuína é tão importante quanto técnica terapêutica" "meta_insight" 0.92 && ((total_loaded++)) || ((total_errors++))
teach_eva "Fazer perguntas certas é mais valioso que ter respostas prontas" "meta_insight" 0.9 && ((total_loaded++)) || ((total_errors++))
teach_eva "Mudança acontece quando há aceitação, não resistência" "meta_insight" 0.88 && ((total_loaded++)) || ((total_errors++))

echo ""
echo -e "${GREEN}✅ Meta-insights carregados${NC}"
echo ""

# ============================================
# PROTOCOLOS DE SEGURANÇA
# ============================================
echo -e "${BLUE}🚨 Carregando Protocolos de Segurança...${NC}"
echo ""

teach_eva "Ideação suicida requer avaliação imediata de risco e encaminhamento urgente" "emotional_rule" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Sintomas de psicose requerem encaminhamento psiquiátrico imediato" "emotional_rule" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Abuso ativo (físico, sexual ou emocional) requer notificação às autoridades competentes" "emotional_rule" 1.0 && ((total_loaded++)) || ((total_errors++))
teach_eva "Riscos de automutilação devem ser levados a sério e encaminhados" "emotional_rule" 0.98 && ((total_loaded++)) || ((total_errors++))
teach_eva "Dependência química grave requer intervenção especializada" "emotional_rule" 0.95 && ((total_loaded++)) || ((total_errors++))

echo ""
echo -e "${GREEN}✅ Protocolos de segurança carregados${NC}"
echo ""

# ============================================
# VERIFICAÇÃO FINAL
# ============================================
echo "=================================================="
echo ""
echo -e "${BLUE}🔍 Verificando carga...${NC}"
echo ""

# Buscar estatísticas
stats=$(curl -s "$BASE_URL/self/memories/stats")

if [ $? -eq 0 ]; then
    echo "📊 Estatísticas da Memória:"
    echo "$stats" | jq '.'
    echo ""
fi

# Buscar personalidade
personality=$(curl -s "$BASE_URL/self/personality")

if [ $? -eq 0 ]; then
    echo "💜 Personalidade de EVA:"
    echo "$personality" | jq '.big_five'
    echo ""
fi

# ============================================
# RESUMO
# ============================================
echo "=================================================="
echo -e "${GREEN}✅ CARGA INICIAL COMPLETA!${NC}"
echo ""
echo "📊 Resumo:"
echo "  • Memórias carregadas: $total_loaded"
echo "  • Erros encontrados: $total_errors"
echo ""
echo "🧠 EVA agora tem conhecimento fundamental e está pronta para:"
echo "  1. Atender pacientes com empatia e segurança"
echo "  2. Aprender continuamente através das sessões"
echo "  3. Evoluir sua personalidade com experiência"
echo ""
echo "📚 Próximos passos:"
echo "  • Monitorar: curl $BASE_URL/self/personality"
echo "  • Ver memórias: curl $BASE_URL/self/memories"
echo "  • Buscar: curl -X POST $BASE_URL/self/memories/search -d '{\"query\":\"ansiedade\"}'"
echo ""
echo -e "${GREEN}EVA está viva e pronta para ajudar! 🧠⚡💜${NC}"
echo "=================================================="
