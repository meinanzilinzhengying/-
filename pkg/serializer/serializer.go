// Package serializer 提供序列化和反序列化工具函数
package serializer

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/binary"
	"encoding/gob"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"

	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

// ==================== JSON序列化 ====================

var jsonEncoderPool = sync.Pool{
	New: func() interface{} {
		return &bytes.Buffer{}
	},
}

// JSONMarshal JSON序列化
func JSONMarshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// JSONMarshalIndent 带缩进的JSON序列化
func JSONMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return json.MarshalIndent(v, prefix, indent)
}

// JSONUnmarshal JSON反序列化
func JSONUnmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// JSONMarshalToString JSON序列化为字符串
func JSONMarshalToString(v interface{}) (string, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// JSONUnmarshalFromString 从字符串JSON反序列化
func JSONUnmarshalFromString(s string, v interface{}) error {
	return json.Unmarshal([]byte(s), v)
}

// JSONMarshalSafe 安全的JSON序列化（忽略错误）
func JSONMarshalSafe(v interface{}) []byte {
	data, _ := json.Marshal(v)
	return data
}

// JSONMarshalSafeString 安全的JSON序列化为字符串
func JSONMarshalSafeString(v interface{}) string {
	data, _ := json.Marshal(v)
	return string(data)
}

// ==================== YAML序列化 ====================

// YAMLMarshal YAML序列化
func YAMLMarshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

// YAMLUnmarshal YAML反序列化
func YAMLUnmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

// YAMLMarshalToString YAML序列化为字符串
func YAMLMarshalToString(v interface{}) (string, error) {
	data, err := yaml.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// YAMLUnmarshalFromString 从字符串YAML反序列化
func YAMLUnmarshalFromString(s string, v interface{}) error {
	return yaml.Unmarshal([]byte(s), v)
}

// ==================== XML序列化 ====================

// XMLMarshal XML序列化
func XMLMarshal(v interface{}) ([]byte, error) {
	return xml.Marshal(v)
}

// XMLMarshalIndent 带缩进的XML序列化
func XMLMarshalIndent(v interface{}, prefix, indent string) ([]byte, error) {
	return xml.MarshalIndent(v, prefix, indent)
}

// XMLUnmarshal XML反序列化
func XMLUnmarshal(data []byte, v interface{}) error {
	return xml.Unmarshal(data, v)
}

// ==================== Protobuf序列化 ====================

// ProtobufMarshal Protobuf序列化
func ProtobufMarshal(v proto.Message) ([]byte, error) {
	return proto.Marshal(v)
}

// ProtobufUnmarshal Protobuf反序列化
func ProtobufUnmarshal(data []byte, v proto.Message) error {
	return proto.Unmarshal(data, v)
}

// ProtobufMarshalToString Protobuf序列化为字符串（Base64编码）
func ProtobufMarshalToString(v proto.Message) (string, error) {
	data, err := proto.Marshal(v)
	if err != nil {
		return "", err
	}
	return Base64Encode(data), nil
}

// ProtobufUnmarshalFromString 从字符串反序列化Protobuf（Base64编码）
func ProtobufUnmarshalFromString(s string, v proto.Message) error {
	data, err := Base64Decode(s)
	if err != nil {
		return err
	}
	return proto.Unmarshal(data, v)
}

// ==================== Gob序列化 ====================

var gobEncoderCache sync.Map

// GobMarshal Gob序列化
func GobMarshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GobUnmarshal Gob反序列化
func GobUnmarshal(data []byte, v interface{}) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)
	return dec.Decode(v)
}

// RegisterGobType 注册Gob类型
func RegisterGobType(v interface{}) {
	gob.Register(v)
}

// ==================== 二进制序列化 ====================

// BinaryMarshal 二进制序列化（使用binary包）
func BinaryMarshal(v interface{}, order binary.ByteOrder) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, order, v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BinaryUnmarshal 二进制反序列化
func BinaryUnmarshal(data []byte, v interface{}, order binary.ByteOrder) error {
	buf := bytes.NewReader(data)
	return binary.Read(buf, order, v)
}

// BinaryMarshalLittleEndian 小端序二进制序列化
func BinaryMarshalLittleEndian(v interface{}) ([]byte, error) {
	return BinaryMarshal(v, binary.LittleEndian)
}

// BinaryUnmarshalLittleEndian 小端序二进制反序列化
func BinaryUnmarshalLittleEndian(data []byte, v interface{}) error {
	return BinaryUnmarshal(data, v, binary.LittleEndian)
}

// BinaryMarshalBigEndian 大端序二进制序列化
func BinaryMarshalBigEndian(v interface{}) ([]byte, error) {
	return BinaryMarshal(v, binary.BigEndian)
}

// BinaryUnmarshalBigEndian 大端序二进制反序列化
func BinaryUnmarshalBigEndian(data []byte, v interface{}) error {
	return BinaryUnmarshal(data, v, binary.BigEndian)
}

// ==================== 通用序列化接口 ====================

// Serializer 序列化接口
type Serializer interface {
	Marshal(v interface{}) ([]byte, error)
	Unmarshal(data []byte, v interface{}) error
	MarshalToString(v interface{}) (string, error)
	UnmarshalFromString(s string, v interface{}) error
}

// Format 序列化格式
type Format string

const (
	FormatJSON     Format = "json"
	FormatYAML     Format = "yaml"
	FormatXML      Format = "xml"
	FormatProtobuf Format = "protobuf"
	FormatGob      Format = "gob"
	FormatBinary   Format = "binary"
)

// GetSerializer 获取指定格式的序列化器
func GetSerializer(format Format) (Serializer, error) {
	switch format {
	case FormatJSON:
		return &JSONSerializer{}, nil
	case FormatYAML:
		return &YAMLSerializer{}, nil
	case FormatXML:
		return &XMLSerializer{}, nil
	case FormatProtobuf:
		return &ProtobufSerializer{}, nil
	case FormatGob:
		return &GobSerializer{}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// JSONSerializer JSON序列化器
type JSONSerializer struct{}

func (s *JSONSerializer) Marshal(v interface{}) ([]byte, error)     { return JSONMarshal(v) }
func (s *JSONSerializer) Unmarshal(data []byte, v interface{}) error { return JSONUnmarshal(data, v) }
func (s *JSONSerializer) MarshalToString(v interface{}) (string, error) {
	return JSONMarshalToString(v)
}
func (s *JSONSerializer) UnmarshalFromString(str string, v interface{}) error {
	return JSONUnmarshalFromString(str, v)
}

// YAMLSerializer YAML序列化器
type YAMLSerializer struct{}

func (s *YAMLSerializer) Marshal(v interface{}) ([]byte, error)     { return YAMLMarshal(v) }
func (s *YAMLSerializer) Unmarshal(data []byte, v interface{}) error { return YAMLUnmarshal(data, v) }
func (s *YAMLSerializer) MarshalToString(v interface{}) (string, error) {
	return YAMLMarshalToString(v)
}
func (s *YAMLSerializer) UnmarshalFromString(str string, v interface{}) error {
	return YAMLUnmarshalFromString(str, v)
}

// XMLSerializer XML序列化器
type XMLSerializer struct{}

func (s *XMLSerializer) Marshal(v interface{}) ([]byte, error)     { return XMLMarshal(v) }
func (s *XMLSerializer) Unmarshal(data []byte, v interface{}) error { return XMLUnmarshal(data, v) }
func (s *XMLSerializer) MarshalToString(v interface{}) (string, error) {
	data, err := XMLMarshal(v)
	return string(data), err
}
func (s *XMLSerializer) UnmarshalFromString(str string, v interface{}) error {
	return XMLUnmarshal([]byte(str), v)
}

// ProtobufSerializer Protobuf序列化器
type ProtobufSerializer struct{}

func (s *ProtobufSerializer) Marshal(v interface{}) ([]byte, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return nil, errors.New("v must implement proto.Message")
	}
	return ProtobufMarshal(msg)
}
func (s *ProtobufSerializer) Unmarshal(data []byte, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("v must implement proto.Message")
	}
	return ProtobufUnmarshal(data, msg)
}
func (s *ProtobufSerializer) MarshalToString(v interface{}) (string, error) {
	msg, ok := v.(proto.Message)
	if !ok {
		return "", errors.New("v must implement proto.Message")
	}
	return ProtobufMarshalToString(msg)
}
func (s *ProtobufSerializer) UnmarshalFromString(str string, v interface{}) error {
	msg, ok := v.(proto.Message)
	if !ok {
		return errors.New("v must implement proto.Message")
	}
	return ProtobufUnmarshalFromString(str, msg)
}

// GobSerializer Gob序列化器
type GobSerializer struct{}

func (s *GobSerializer) Marshal(v interface{}) ([]byte, error)     { return GobMarshal(v) }
func (s *GobSerializer) Unmarshal(data []byte, v interface{}) error { return GobUnmarshal(data, v) }
func (s *GobSerializer) MarshalToString(v interface{}) (string, error) {
	data, err := GobMarshal(v)
	if err != nil {
		return "", err
	}
	return Base64Encode(data), nil
}
func (s *GobSerializer) UnmarshalFromString(str string, v interface{}) error {
	data, err := Base64Decode(str)
	if err != nil {
		return err
	}
	return GobUnmarshal(data, v)
}

// ==================== 类型转换 ====================

// MapToStruct 将map转换为结构体
func MapToStruct(m map[string]interface{}, v interface{}) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// StructToMap 将结构体转换为map
func StructToMap(v interface{}) (map[string]interface{}, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return m, nil
}

// StructToMapString 将结构体转换为map[string]string
func StructToMapString(v interface{}) (map[string]string, error) {
	m, err := StructToMap(v)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string)
	for k, v := range m {
		result[k] = fmt.Sprintf("%v", v)
	}
	return result, nil
}

// DeepCopy 深拷贝
func DeepCopy(src, dst interface{}) error {
	data, err := GobMarshal(src)
	if err != nil {
		return err
	}
	return GobUnmarshal(data, dst)
}

// DeepCopyJSON 使用JSON进行深拷贝
func DeepCopyJSON(src, dst interface{}) error {
	data, err := json.Marshal(src)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dst)
}

// ==================== 流式处理 ====================

// JSONEncoder JSON编码器
type JSONEncoder struct {
	encoder *json.Encoder
}

// NewJSONEncoder 创建JSON编码器
func NewJSONEncoder(w io.Writer) *JSONEncoder {
	return &JSONEncoder{encoder: json.NewEncoder(w)}
}

func (e *JSONEncoder) Encode(v interface{}) error {
	return e.encoder.Encode(v)
}

// JSONDecoder JSON解码器
type JSONDecoder struct {
	decoder *json.Decoder
}

// NewJSONDecoder 创建JSON解码器
func NewJSONDecoder(r io.Reader) *JSONDecoder {
	return &JSONDecoder{decoder: json.NewDecoder(r)}
}

func (d *JSONDecoder) Decode(v interface{}) error {
	return d.decoder.Decode(v)
}

// ==================== 压缩序列化 ====================

// CompressAndSerialize 压缩并序列化
func CompressAndSerialize(v interface{}, format Format) ([]byte, error) {
	data, err := Serialize(v, format)
	if err != nil {
		return nil, err
	}
	return Compress(data)
}

// DecompressAndDeserialize 解压并反序列化
func DecompressAndDeserialize(data []byte, v interface{}, format Format) error {
	decompressed, err := Decompress(data)
	if err != nil {
		return err
	}
	return Deserialize(decompressed, v, format)
}

// Serialize 序列化
func Serialize(v interface{}, format Format) ([]byte, error) {
	s, err := GetSerializer(format)
	if err != nil {
		return nil, err
	}
	return s.Marshal(v)
}

// Deserialize 反序列化
func Deserialize(data []byte, v interface{}, format Format) error {
	s, err := GetSerializer(format)
	if err != nil {
		return err
	}
	return s.Unmarshal(data, v)
}

// ==================== 辅助函数 ====================

// IsEmpty 检查是否为空
func IsEmpty(v interface{}) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Ptr, reflect.Interface:
		return rv.IsNil()
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return rv.Len() == 0
	case reflect.Bool:
		return !rv.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	}
	return false
}

// GetTypeName 获取类型名称
func GetTypeName(v interface{}) string {
	if v == nil {
		return "nil"
	}
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// GetJSONFieldName 获取JSON字段名
func GetJSONFieldName(t reflect.Type, fieldName string) string {
	field, found := t.FieldByName(fieldName)
	if !found {
		return fieldName
	}
	tag := field.Tag.Get("json")
	if tag == "" {
		return fieldName
	}
	parts := strings.Split(tag, ",")
	if parts[0] == "-" {
		return ""
	}
	if parts[0] != "" {
		return parts[0]
	}
	return fieldName
}

// MergeMaps 合并多个map
func MergeMaps(maps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for _, m := range maps {
		for k, v := range m {
			result[k] = v
		}
	}
	return result
}

// FilterMap 过滤map
func FilterMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	result := make(map[string]interface{})
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for k, v := range m {
		if keySet[k] {
			result[k] = v
		}
	}
	return result
}

// OmitMap 排除指定key的map
func OmitMap(m map[string]interface{}, keys ...string) map[string]interface{} {
	result := make(map[string]interface{})
	keySet := make(map[string]bool)
	for _, k := range keys {
		keySet[k] = true
	}
	for k, v := range m {
		if !keySet[k] {
			result[k] = v
		}
	}
	return result
}

// Base64Encode Base64编码
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64Decode Base64解码
func Base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// Compress 压缩数据（使用gzip）
func Compress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decompress 解压数据
func Decompress(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	return io.ReadAll(reader)
}
