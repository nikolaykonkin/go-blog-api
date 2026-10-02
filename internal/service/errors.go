package service

import "errors"

// ErrValidation оборачивает все ошибки валидации входных данных
// Хендлеры проверяют его через errors.Is, чтобы отличить 400 от 500 и не сравнивать тексты ошибок
var ErrValidation = errors.New("validation error")
