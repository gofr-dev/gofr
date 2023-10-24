package errors

// InvalidFileStorage is used to show invalid file store config is provided
const InvalidFileStorage = Error("Invalid File Storage.Please set a valid value of FILE_STORE:{LOCAL or AZURE or GCP or AWS}")
