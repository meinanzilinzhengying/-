// Package jvmmem Java符号表管理
// Copyright (c) 2026 Cloud Flow Team
// Licensed under the MIT License.

package jvmmem

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// JavaSymbol Java符号信息
type JavaSymbol struct {
	Address    uint64
	ClassName  string
	MethodName string
	FileName   string
	LineNumber int
	Signature  string
}

// JavaSymbolTable Java符号表
type JavaSymbolTable struct {
	symbols map[uint32]map[uint64]*JavaSymbol // pid -> (address -> symbol)
	classes map[uint32]map[uint32]string      // pid -> (class_id -> class_name)
	methods map[uint32]map[uint32]string      // pid -> (method_id -> method_name)
	mu      sync.RWMutex
}

// NewJavaSymbolTable 创建Java符号表
func NewJavaSymbolTable() *JavaSymbolTable {
	return &JavaSymbolTable{
		symbols: make(map[uint32]map[uint64]*JavaSymbol),
		classes: make(map[uint32]map[uint32]string),
		methods: make(map[uint32]map[uint32]string),
	}
}

// Load 加载指定PID的符号表
func (st *JavaSymbolTable) Load(pid uint32) error {
	st.mu.Lock()
	defer st.mu.Unlock()

	// 初始化pid的map
	st.symbols[pid] = make(map[uint64]*JavaSymbol)
	st.classes[pid] = make(map[uint32]string)
	st.methods[pid] = make(map[uint32]string)

	// 加载perf-map文件
	if err := st.loadPerfMap(pid); err != nil {
		// perf-map可能不存在，继续尝试其他方式
	}

	// 加载class dump
	if err := st.loadClassDump(pid); err != nil {
		// class dump可能不存在
	}

	return nil
}

// Unload 卸载指定PID的符号表
func (st *JavaSymbolTable) Unload(pid uint32) {
	st.mu.Lock()
	defer st.mu.Unlock()

	delete(st.symbols, pid)
	delete(st.classes, pid)
	delete(st.methods, pid)
}

// Resolve 解析地址对应的符号
func (st *JavaSymbolTable) Resolve(pid uint32, addr uint64) *JavaSymbol {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidSymbols, ok := st.symbols[pid]; ok {
		if sym, ok := pidSymbols[addr]; ok {
			return sym
		}

		// 尝试查找最近的符号（在范围内）
		for symAddr, sym := range pidSymbols {
			if addr >= symAddr && addr < symAddr+4096 {
				// 返回一个副本，调整偏移
				result := *sym
				result.Address = addr
				return &result
			}
		}
	}

	return nil
}

// GetClassName 获取类名
func (st *JavaSymbolTable) GetClassName(pid, classID uint32) string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidClasses, ok := st.classes[pid]; ok {
		if name, ok := pidClasses[classID]; ok {
			return name
		}
	}

	return fmt.Sprintf("Class_%d", classID)
}

// GetMethodName 获取方法名
func (st *JavaSymbolTable) GetMethodName(pid, methodID uint32) string {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidMethods, ok := st.methods[pid]; ok {
		if name, ok := pidMethods[methodID]; ok {
			return name
		}
	}

	return fmt.Sprintf("method_%d", methodID)
}

// loadPerfMap 加载perf-map文件
func (st *JavaSymbolTable) loadPerfMap(pid uint32) error {
	mapFile := fmt.Sprintf("/tmp/perf-%d.map", pid)
	file, err := os.Open(mapFile)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if err := st.parsePerfMapLine(pid, line); err != nil {
			continue
		}
	}

	return scanner.Err()
}

// parsePerfMapLine 解析perf-map行
// 格式: <start_addr> <size> <symbol_name>
func (st *JavaSymbolTable) parsePerfMapLine(pid uint32, line string) error {
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return fmt.Errorf("invalid line format")
	}

	addr, err := strconv.ParseUint(fields[0], 16, 64)
	if err != nil {
		return err
	}

	symbolName := strings.Join(fields[2:], " ")

	// 解析Java符号名
	// 格式: Ljava/lang/String;.<init>
	className, methodName := st.parseJavaSymbol(symbolName)

	sym := &JavaSymbol{
		Address:    addr,
		ClassName:  className,
		MethodName: methodName,
		Signature:  symbolName,
	}

	st.symbols[pid][addr] = sym
	return nil
}

// parseJavaSymbol 解析Java符号名
func (st *JavaSymbolTable) parseJavaSymbol(symbol string) (className, methodName string) {
	// 尝试解析常见的Java符号格式
	
	// 格式1: Ljava/lang/String;.<init>
	if idx := strings.Index(symbol, ";."); idx > 0 {
		className = symbol[1:idx]
		className = strings.ReplaceAll(className, "/", ".")
		if idx+2 < len(symbol) {
			methodName = symbol[idx+2:]
		}
		return
	}

	// 格式2: java/lang/String.<init>
	if idx := strings.LastIndex(symbol, "."); idx > 0 {
		className = symbol[:idx]
		className = strings.ReplaceAll(className, "/", ".")
		methodName = symbol[idx+1:]
		return
	}

	// 格式3: 包含::的C++风格
	if idx := strings.Index(symbol, "::"); idx > 0 {
		className = symbol[:idx]
		methodName = symbol[idx+2:]
		return
	}

	// 默认：整个作为方法名
	return "<unknown>", symbol
}

// loadClassDump 加载class dump
func (st *JavaSymbolTable) loadClassDump(pid uint32) error {
	// 从/proc/<pid>/root/tmp/或/tmp/加载class dump
	dumpPaths := []string{
		fmt.Sprintf("/proc/%d/root/tmp/klass_dump.log", pid),
		fmt.Sprintf("/tmp/klass_dump_%d.log", pid),
	}

	for _, path := range dumpPaths {
		if _, err := os.Stat(path); err == nil {
			return st.parseClassDump(pid, path)
		}
	}

	return fmt.Errorf("no class dump found")
}

// parseClassDump 解析class dump文件
func (st *JavaSymbolTable) parseClassDump(pid uint32, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// 解析class dump格式
		// 格式示例: 0x12345678 java.lang.String
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			addr, err := strconv.ParseUint(fields[0], 0, 32)
			if err == nil {
				className := fields[1]
				st.classes[pid][uint32(addr)] = className
			}
		}
	}

	return scanner.Err()
}

// UpdateClassInfo 更新类信息
func (st *JavaSymbolTable) UpdateClassInfo(pid, classID uint32, className string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.classes[pid]; !ok {
		st.classes[pid] = make(map[uint32]string)
	}
	st.classes[pid][classID] = className
}

// UpdateMethodInfo 更新方法信息
func (st *JavaSymbolTable) UpdateMethodInfo(pid, methodID uint32, methodName string) {
	st.mu.Lock()
	defer st.mu.Unlock()

	if _, ok := st.methods[pid]; !ok {
		st.methods[pid] = make(map[uint32]string)
	}
	st.methods[pid][methodID] = methodName
}

// GetAllSymbols 获取指定PID的所有符号
func (st *JavaSymbolTable) GetAllSymbols(pid uint32) []*JavaSymbol {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidSymbols, ok := st.symbols[pid]; ok {
		result := make([]*JavaSymbol, 0, len(pidSymbols))
		for _, sym := range pidSymbols {
			result = append(result, sym)
		}
		return result
	}

	return nil
}

// GetClassCount 获取类数量
func (st *JavaSymbolTable) GetClassCount(pid uint32) int {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidClasses, ok := st.classes[pid]; ok {
		return len(pidClasses)
	}
	return 0
}

// GetMethodCount 获取方法数量
func (st *JavaSymbolTable) GetMethodCount(pid uint32) int {
	st.mu.RLock()
	defer st.mu.RUnlock()

	if pidMethods, ok := st.methods[pid]; ok {
		return len(pidMethods)
	}
	return 0
}
