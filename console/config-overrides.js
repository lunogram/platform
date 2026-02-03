module.exports = function override(config) {
    config.ignoreWarnings = [
        {
            message: /source-map-loader/,
            module: /node_modules\/rrule/,
        },
    ]
    return config
}
