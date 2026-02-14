#!/bin/bash

# Script para corrigir todos os erros de compilação de uma vez
# Remove funções duplicadas e corrige chamadas de função

echo "🔧 Corrigindo erros de compilação..."

# Backup dos arquivos
echo "📦 Criando backup..."
cp -r internal/cortex/personality internal/cortex/personality.backup

# Remove containsString duplicado de personality_router.go
echo "✅ Removendo containsString de personality_router.go..."
sed -i '/^func containsString(slice \[\]string, item string) bool {$/,/^}$/d' internal/cortex/personality/personality_router.go

# Remove contains de interpretation_validator.go
echo "✅ Removendo contains de interpretation_validator.go..."
sed -i '/^func contains(text string, keywords \[\]string) bool {$/,/^}$/d' internal/cortex/personality/interpretation_validator.go

# Remove contains de situation_modulator.go  
echo "✅ Removendo contains de situation_modulator.go..."
sed -i '/^func contains(slice \[\]string, item string) bool {$/,/^}$/d' internal/cortex/personality/situation_modulator.go

# Remove min de judge_quality_tracker.go
echo "✅ Removendo min/abs de judge_quality_tracker.go..."
sed -i '/^func min(a, b float64) float64 {$/,/^}$/d' internal/cortex/personality/judge_quality_tracker.go
sed -i '/^func abs(x float64) float64 {$/,/^}$/d' internal/cortex/personality/judge_quality_tracker.go

# Remove funções duplicadas de trajectory_analyzer.go
echo "✅ Removendo funções duplicadas de trajectory_analyzer.go..."
sed -i '/^func variance(values \[\]float64) float64 {$/,/^}$/d' internal/cortex/personality/trajectory_analyzer.go
sed -i '/^func average(values \[\]float64) float64 {$/,/^}$/d' internal/cortex/personality/trajectory_analyzer.go
sed -i '/^func min(a, b int) int {$/,/^}$/d' internal/cortex/personality/trajectory_analyzer.go

# Remove funções duplicadas de target_quality_assessor.go
echo "✅ Removendo funções duplicadas de target_quality_assessor.go..."
sed -i '/^func variance(values \[\]float64) float64 {$/,/^}$/d' internal/cortex/personality/target_quality_assessor.go
sed -i '/^func average(values \[\]float64) float64 {$/,/^}$/d' internal/cortex/personality/target_quality_assessor.go

echo "✅ Correções aplicadas!"
echo "🧪 Testando compilação..."
go build ./internal/cortex/personality/...

if [ $? -eq 0 ]; then
    echo "✅ Compilação bem-sucedida!"
    rm -rf internal/cortex/personality.backup
else
    echo "❌ Ainda há erros. Restaurando backup..."
    rm -rf internal/cortex/personality
    mv internal/cortex/personality.backup internal/cortex/personality
fi
