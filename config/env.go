package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// loadFromEnv loads configuration values from environment variables
func loadFromEnv(cfg *Config) error {
	return loadFromEnvRecursive(cfg, "")
}

// loadFromEnvRecursive recursively loads environment variables into a struct
func loadFromEnvRecursive(v interface{}, prefix string) error {
	val := reflect.ValueOf(v)
	if val.Kind() != reflect.Ptr {
		return fmt.Errorf("expected pointer, got %s", val.Kind())
	}

	val = val.Elem()
	if val.Kind() != reflect.Struct {
		return fmt.Errorf("expected struct, got %s", val.Kind())
	}

	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		// Recurse into nested structs to honor their env tags
		if field.Kind() == reflect.Struct {
			if field.CanAddr() {
				if err := loadFromEnvRecursive(field.Addr().Interface(), prefix); err != nil {
					return err
				}
			}
			continue
		}

		// Get the env tag
		envTag := fieldType.Tag.Get("env")
		if envTag == "" {
			continue
		}

		// Build full environment variable name
		envVar := envTag
		if prefix != "" {
			envVar = prefix + "_" + envTag
		}

		// Get environment variable value
		envValue := os.Getenv(envVar)
		if envValue == "" {
			continue // Skip if not set
		}

		// Set the field value based on its type
		if err := setFieldValue(field, fieldType, envValue); err != nil {
			return fmt.Errorf("failed to set field %s from env var %s: %w", fieldType.Name, envVar, err)
		}
	}

	return nil
}

// setFieldValue sets a struct field from an environment variable string
func setFieldValue(field reflect.Value, fieldType reflect.StructField, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field %s is not settable", fieldType.Name)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)

	case reflect.Bool:
		boolVal, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("invalid boolean value: %s", value)
		}
		field.SetBool(boolVal)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if fieldType.Type == reflect.TypeOf(time.Duration(0)) {
			// Handle time.Duration specially
			duration, err := time.ParseDuration(value)
			if err != nil {
				return fmt.Errorf("invalid duration value: %s", value)
			}
			field.SetInt(int64(duration))
		} else {
			intVal, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid integer value: %s", value)
			}
			field.SetInt(intVal)
		}

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		uintVal, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return fmt.Errorf("invalid unsigned integer value: %s", value)
		}
		field.SetUint(uintVal)

	case reflect.Float32, reflect.Float64:
		floatVal, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("invalid float value: %s", value)
		}
		field.SetFloat(floatVal)

	case reflect.Slice:
		if fieldType.Type.Elem().Kind() == reflect.String {
			// Handle string slices (comma-separated)
			parts := strings.Split(value, ",")
			slice := reflect.MakeSlice(fieldType.Type, len(parts), len(parts))
			for i, part := range parts {
				slice.Index(i).SetString(strings.TrimSpace(part))
			}
			field.Set(slice)
		} else {
			return fmt.Errorf("unsupported slice type: %s", fieldType.Type.Elem().Kind())
		}

	case reflect.Map:
		if fieldType.Type.Key().Kind() == reflect.String && fieldType.Type.Elem().Kind() == reflect.String {
			// Handle string-to-string maps (key=value,key2=value2 format)
			pairs := strings.Split(value, ",")
			mapVal := reflect.MakeMap(fieldType.Type)

			for _, pair := range pairs {
				kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
				if len(kv) != 2 {
					return fmt.Errorf("invalid map entry format: %s", pair)
				}
				mapVal.SetMapIndex(reflect.ValueOf(kv[0]), reflect.ValueOf(kv[1]))
			}

			field.Set(mapVal)
		} else {
			return fmt.Errorf("unsupported map type: %s -> %s", fieldType.Type.Key().Kind(), fieldType.Type.Elem().Kind())
		}

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}
