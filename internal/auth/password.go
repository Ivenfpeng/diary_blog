package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

const (
	argonMemory      uint32 = 64 * 1024
	argonIterations  uint32 = 3
	argonParallelism uint8  = 2
	argonSaltLength         = 16
	argonKeyLength   uint32 = 32
)

var ErrInvalidPasswordHash = errors.New("invalid password hash")

func HashPassword(password string) (string, error) {
	salt := make([]byte, argonSaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate password salt: %w", err)
	}
	key := argon2.IDKey([]byte(password), salt, argonIterations, argonMemory, argonParallelism, argonKeyLength)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonIterations, argonParallelism,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key)), nil
}

func VerifyPassword(password, encodedHash string) (bool, error) {
	memory, iterations, parallelism, salt, expected, err := parsePasswordHash(encodedHash)
	if err != nil {
		return false, err
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1, nil
}

func parsePasswordHash(encodedHash string) (uint32, uint32, uint8, []byte, []byte, error) {
	if len(encodedHash) > 256 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" || parts[2] != fmt.Sprintf("v=%d", argon2.Version) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	parameters := strings.Split(parts[3], ",")
	if len(parameters) != 3 {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	memory64, err := parseParameter(parameters[0], "m", 32)
	if err != nil || memory64 == 0 || memory64 > uint64(argonMemory) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	iterations64, err := parseParameter(parameters[1], "t", 32)
	if err != nil || iterations64 == 0 || iterations64 > uint64(argonIterations) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	parallelism64, err := parseParameter(parameters[2], "p", 8)
	if err != nil || parallelism64 == 0 || parallelism64 > uint64(argonParallelism) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	if len(parts[4]) != base64.RawStdEncoding.EncodedLen(argonSaltLength) || len(parts[5]) != base64.RawStdEncoding.EncodedLen(int(argonKeyLength)) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != argonSaltLength {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(expected) != int(argonKeyLength) {
		return 0, 0, 0, nil, nil, ErrInvalidPasswordHash
	}
	return uint32(memory64), uint32(iterations64), uint8(parallelism64), salt, expected, nil
}

func parseParameter(value, name string, bitSize int) (uint64, error) {
	prefix := name + "="
	if !strings.HasPrefix(value, prefix) {
		return 0, ErrInvalidPasswordHash
	}
	parsed, err := strconv.ParseUint(strings.TrimPrefix(value, prefix), 10, bitSize)
	if err != nil {
		return 0, ErrInvalidPasswordHash
	}
	return parsed, nil
}
