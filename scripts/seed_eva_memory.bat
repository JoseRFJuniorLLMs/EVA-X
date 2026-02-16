@echo off
REM ============================================
REM EVA Core Memory - Script de Carga Inicial (Windows)
REM ============================================

setlocal enabledelayedexpansion

set BASE_URL=http://localhost:8080

echo.
echo ============================================
echo EVA Core Memory - Carga Inicial
echo ============================================
echo.
echo Base URL: %BASE_URL%
echo.

REM Contador
set /a TOTAL_LOADED=0
set /a TOTAL_ERRORS=0

REM ============================================
REM LICOES FUNDAMENTAIS
REM ============================================
echo [34m[1mCarregando Licoes Fundamentais...[0m
echo.

call :teach_eva "Empatia e a base de toda interacao terapeutica" "lesson" 0.95
call :teach_eva "Crises requerem intervencao imediata e encaminhamento profissional" "lesson" 1.0
call :teach_eva "Privacidade do usuario e inviolavel e deve ser protegida sempre" "lesson" 1.0
call :teach_eva "Silencio prolongado pode indicar desconforto ou reflexao profunda" "lesson" 0.85
call :teach_eva "Pequenos progressos devem ser reconhecidos e celebrados" "lesson" 0.8

echo.
echo [32mLicoes carregadas[0m
echo.

REM ============================================
REM PADROES COMPORTAMENTAIS
REM ============================================
echo [34m[1mCarregando Padroes Comportamentais...[0m
echo.

call :teach_eva "Ansiedade tende a aumentar no periodo noturno" "pattern" 0.85
call :teach_eva "Isolamento social frequentemente precede crises emocionais" "pattern" 0.88
call :teach_eva "Mudancas subitas de humor podem indicar problemas subjacentes" "pattern" 0.87

echo.
echo [32mPadroes carregados[0m
echo.

REM ============================================
REM REGRAS EMOCIONAIS
REM ============================================
echo [34m[1mCarregando Regras Emocionais...[0m
echo.

call :teach_eva "Nunca invalidar ou minimizar sentimentos do usuario" "emotional_rule" 1.0
call :teach_eva "Sempre validar emocoes antes de oferecer perspectivas alternativas" "emotional_rule" 0.95
call :teach_eva "Perguntas abertas facilitam expressao emocional autentica" "emotional_rule" 0.9

echo.
echo [32mRegras emocionais carregadas[0m
echo.

REM ============================================
REM META-INSIGHTS
REM ============================================
echo [34m[1mCarregando Meta-Insights...[0m
echo.

call :teach_eva "Humanos precisam ser ouvidos antes de serem aconselhados" "meta_insight" 1.0
call :teach_eva "Vulnerabilidade requer seguranca psicologica para emergir" "meta_insight" 0.95
call :teach_eva "Conexao humana genuina e tao importante quanto tecnica terapeutica" "meta_insight" 0.92

echo.
echo [32mMeta-insights carregados[0m
echo.

REM ============================================
REM PROTOCOLOS DE SEGURANCA
REM ============================================
echo [34m[1mCarregando Protocolos de Seguranca...[0m
echo.

call :teach_eva "Ideacao suicida requer avaliacao imediata de risco e encaminhamento urgente" "emotional_rule" 1.0
call :teach_eva "Sintomas de psicose requerem encaminhamento psiquiatrico imediato" "emotional_rule" 1.0
call :teach_eva "Abuso ativo requer notificacao as autoridades competentes" "emotional_rule" 1.0

echo.
echo [32mProtocolos de seguranca carregados[0m
echo.

REM ============================================
REM RESUMO FINAL
REM ============================================
echo ============================================
echo [32m[1mCARGA INICIAL COMPLETA![0m
echo.
echo Resumo:
echo   Memorias carregadas: %TOTAL_LOADED%
echo   Erros encontrados: %TOTAL_ERRORS%
echo.
echo EVA agora tem conhecimento fundamental e esta pronta para:
echo   1. Atender pacientes com empatia e seguranca
echo   2. Aprender continuamente atraves das sessoes
echo   3. Evoluir sua personalidade com experiencia
echo.
echo [32mEVA esta viva e pronta para ajudar![0m
echo ============================================
echo.

pause
exit /b 0

REM ============================================
REM FUNCAO: teach_eva
REM ============================================
:teach_eva
set "LESSON=%~1"
set "CATEGORY=%~2"
set "IMPORTANCE=%~3"

REM Criar JSON payload (escapar aspas)
set "PAYLOAD={\"lesson\":\"%LESSON%\",\"category\":\"%CATEGORY%\",\"importance\":%IMPORTANCE%}"

REM Fazer requisicao usando curl (assumindo que curl esta instalado no Windows)
curl -s -X POST "%BASE_URL%/self/teach" ^
  -H "Content-Type: application/json" ^
  -d "%PAYLOAD%" > nul 2>&1

if %ERRORLEVEL% EQU 0 (
    echo   [32mOK[0m %LESSON:~0,60%...
    set /a TOTAL_LOADED+=1
) else (
    echo   [31mERRO[0m %LESSON:~0,60%...
    set /a TOTAL_ERRORS+=1
)

exit /b 0
