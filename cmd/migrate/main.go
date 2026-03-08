// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// migrate was a PostgreSQL migration tool. PostgreSQL has been fully replaced by NietzscheDB.
// All data lives in NietzscheDB collections. This tool is no longer needed.
package main

import "fmt"

func main() {
	fmt.Println("⚠️  Este tool de migração PostgreSQL está obsoleto.")
	fmt.Println("   Todos os dados agora vivem no NietzscheDB.")
	fmt.Println("   Use nietzsche-server diretamente para gerenciar coleções.")
}
