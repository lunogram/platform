/**
 * Maps IANA timezone identifiers to representative [longitude, latitude] coordinates.
 * Used for displaying a user's timezone on a map.
 *
 * This covers all major IANA timezone zones. For unlisted timezones,
 * we fall back to parsing the UTC offset or a sensible default.
 */

const timezoneCoordinates: Record<string, [number, number]> = {
    // Africa
    "Africa/Abidjan": [-4.03, 5.32],
    "Africa/Accra": [-0.19, 5.56],
    "Africa/Addis_Ababa": [38.75, 9.02],
    "Africa/Algiers": [3.06, 36.75],
    "Africa/Cairo": [31.24, 30.04],
    "Africa/Casablanca": [-7.59, 33.59],
    "Africa/Dar_es_Salaam": [39.27, -6.81],
    "Africa/Johannesburg": [28.05, -26.2],
    "Africa/Khartoum": [32.53, 15.6],
    "Africa/Lagos": [3.39, 6.45],
    "Africa/Maputo": [32.57, -25.97],
    "Africa/Nairobi": [36.82, -1.29],
    "Africa/Tunis": [10.17, 36.8],

    // America
    "America/Anchorage": [-149.9, 61.22],
    "America/Argentina/Buenos_Aires": [-58.38, -34.6],
    "America/Bogota": [-74.07, 4.71],
    "America/Buenos_Aires": [-58.38, -34.6],
    "America/Caracas": [-66.9, 10.5],
    "America/Chicago": [-87.63, 41.88],
    "America/Chihuahua": [-106.09, 28.63],
    "America/Costa_Rica": [-84.09, 9.93],
    "America/Denver": [-104.99, 39.74],
    "America/Detroit": [-83.05, 42.33],
    "America/Edmonton": [-113.49, 53.55],
    "America/El_Salvador": [-89.19, 13.69],
    "America/Guatemala": [-90.51, 14.63],
    "America/Halifax": [-63.57, 44.65],
    "America/Havana": [-82.37, 23.11],
    "America/Indiana/Indianapolis": [-86.16, 39.77],
    "America/Jamaica": [-76.79, 18.11],
    "America/Lima": [-77.04, -12.05],
    "America/Los_Angeles": [-118.24, 34.05],
    "America/Managua": [-86.25, 12.13],
    "America/Manaus": [-60.03, -3.12],
    "America/Mexico_City": [-99.13, 19.43],
    "America/Monterrey": [-100.31, 25.67],
    "America/Montevideo": [-56.16, -34.9],
    "America/Montreal": [-73.57, 45.5],
    "America/New_York": [-74.01, 40.71],
    "America/Panama": [-79.52, 8.98],
    "America/Phoenix": [-112.07, 33.45],
    "America/Puerto_Rico": [-66.11, 18.47],
    "America/Regina": [-104.62, 50.45],
    "America/Santiago": [-70.65, -33.45],
    "America/Santo_Domingo": [-69.9, 18.47],
    "America/Sao_Paulo": [-46.63, -23.55],
    "America/St_Johns": [-52.71, 47.56],
    "America/Tegucigalpa": [-87.22, 14.07],
    "America/Tijuana": [-117.02, 32.53],
    "America/Toronto": [-79.38, 43.65],
    "America/Vancouver": [-123.12, 49.28],
    "America/Winnipeg": [-97.14, 49.9],

    // Asia
    "Asia/Almaty": [76.95, 43.24],
    "Asia/Amman": [35.93, 31.95],
    "Asia/Baghdad": [44.37, 33.31],
    "Asia/Baku": [49.87, 40.41],
    "Asia/Bangkok": [100.5, 13.76],
    "Asia/Beirut": [35.5, 33.89],
    "Asia/Calcutta": [88.36, 22.57],
    "Asia/Colombo": [79.86, 6.93],
    "Asia/Damascus": [36.29, 33.51],
    "Asia/Dhaka": [90.41, 23.81],
    "Asia/Dubai": [55.27, 25.2],
    "Asia/Ho_Chi_Minh": [106.63, 10.82],
    "Asia/Hong_Kong": [114.17, 22.28],
    "Asia/Istanbul": [28.98, 41.01],
    "Asia/Jakarta": [106.85, -6.21],
    "Asia/Jerusalem": [35.22, 31.77],
    "Asia/Kabul": [69.17, 34.53],
    "Asia/Karachi": [67.01, 24.86],
    "Asia/Kathmandu": [85.32, 27.72],
    "Asia/Kolkata": [72.88, 19.08],
    "Asia/Kuala_Lumpur": [101.69, 3.14],
    "Asia/Kuwait": [47.98, 29.38],
    "Asia/Magadan": [150.78, 59.56],
    "Asia/Manila": [120.98, 14.6],
    "Asia/Muscat": [58.39, 23.59],
    "Asia/Novosibirsk": [82.93, 55.04],
    "Asia/Riyadh": [46.72, 24.69],
    "Asia/Seoul": [126.98, 37.57],
    "Asia/Shanghai": [121.47, 31.23],
    "Asia/Singapore": [103.82, 1.35],
    "Asia/Taipei": [121.57, 25.03],
    "Asia/Tashkent": [69.28, 41.31],
    "Asia/Tbilisi": [44.83, 41.69],
    "Asia/Tehran": [51.39, 35.69],
    "Asia/Tokyo": [139.69, 35.69],
    "Asia/Vladivostok": [131.89, 43.12],
    "Asia/Yakutsk": [129.73, 62.04],
    "Asia/Yekaterinburg": [60.6, 56.84],
    "Asia/Yerevan": [44.51, 40.18],

    // Atlantic
    "Atlantic/Azores": [-25.67, 37.74],
    "Atlantic/Cape_Verde": [-22.94, 14.92],
    "Atlantic/Reykjavik": [-21.9, 64.15],

    // Australia
    "Australia/Adelaide": [138.6, -34.93],
    "Australia/Brisbane": [153.03, -27.47],
    "Australia/Darwin": [130.84, -12.46],
    "Australia/Hobart": [147.33, -42.88],
    "Australia/Melbourne": [144.96, -37.81],
    "Australia/Perth": [115.86, -31.95],
    "Australia/Sydney": [151.21, -33.87],

    // Europe
    "Europe/Amsterdam": [4.9, 52.37],
    "Europe/Athens": [23.73, 37.98],
    "Europe/Belgrade": [20.46, 44.82],
    "Europe/Berlin": [13.4, 52.52],
    "Europe/Brussels": [4.35, 50.85],
    "Europe/Bucharest": [26.1, 44.43],
    "Europe/Budapest": [19.04, 47.5],
    "Europe/Copenhagen": [12.57, 55.68],
    "Europe/Dublin": [-6.26, 53.35],
    "Europe/Helsinki": [24.94, 60.17],
    "Europe/Istanbul": [28.98, 41.01],
    "Europe/Kyiv": [30.52, 50.45],
    "Europe/Lisbon": [-9.14, 38.74],
    "Europe/London": [-0.13, 51.51],
    "Europe/Madrid": [-3.7, 40.42],
    "Europe/Minsk": [27.55, 53.9],
    "Europe/Moscow": [37.62, 55.76],
    "Europe/Oslo": [10.75, 59.91],
    "Europe/Paris": [2.35, 48.86],
    "Europe/Prague": [14.42, 50.08],
    "Europe/Riga": [24.11, 56.95],
    "Europe/Rome": [12.5, 41.9],
    "Europe/Sarajevo": [18.41, 43.86],
    "Europe/Sofia": [23.32, 42.7],
    "Europe/Stockholm": [18.07, 59.33],
    "Europe/Tallinn": [24.75, 59.44],
    "Europe/Vienna": [16.37, 48.21],
    "Europe/Vilnius": [25.28, 54.69],
    "Europe/Warsaw": [21.01, 52.23],
    "Europe/Zagreb": [15.98, 45.81],
    "Europe/Zurich": [8.54, 47.38],

    // Indian
    "Indian/Maldives": [73.51, 4.18],
    "Indian/Mauritius": [57.5, -20.16],

    // Pacific
    "Pacific/Auckland": [174.78, -36.85],
    "Pacific/Chatham": [-176.46, -43.88],
    "Pacific/Fiji": [178.44, -18.14],
    "Pacific/Guam": [144.79, 13.44],
    "Pacific/Honolulu": [-157.86, 21.31],
    "Pacific/Midway": [-177.37, 28.21],
    "Pacific/Noumea": [166.46, -22.28],
    "Pacific/Pago_Pago": [-170.7, -14.28],
    "Pacific/Port_Moresby": [147.15, -9.44],
    "Pacific/Tongatapu": [-175.2, -21.21],

    // UTC aliases
    UTC: [0, 51.51],
    GMT: [0, 51.51],
    "Etc/UTC": [0, 51.51],
    "Etc/GMT": [0, 51.51],
}

/**
 * Returns the [longitude, latitude] for a given IANA timezone string.
 * Returns null if the timezone cannot be resolved.
 */
export function getTimezoneCoordinates(timezone: string): [number, number] | null {
    // Direct lookup
    if (timezoneCoordinates[timezone]) {
        return timezoneCoordinates[timezone]
    }

    // Try common aliases (e.g. "US/Eastern" -> "America/New_York")
    try {
        // Use Intl to resolve the canonical timezone name
        const formatter = new Intl.DateTimeFormat("en-US", { timeZone: timezone })
        const resolved = formatter.resolvedOptions().timeZone
        if (resolved && timezoneCoordinates[resolved]) {
            return timezoneCoordinates[resolved]
        }
    } catch {
        // Invalid timezone
    }

    return null
}

/**
 * Formats a timezone string for display.
 * e.g. "America/New_York" -> "America / New York"
 */
export function formatTimezone(timezone: string): string {
    return timezone.replace(/_/g, " ").replace(/\//g, " / ")
}

/**
 * Returns the current local time string in the given timezone.
 */
export function getLocalTimeInTimezone(timezone: string): string | null {
    try {
        return new Intl.DateTimeFormat("en-US", {
            timeZone: timezone,
            hour: "numeric",
            minute: "2-digit",
            hour12: true,
        }).format(new Date())
    } catch {
        return null
    }
}

/**
 * Returns the UTC offset string for a given timezone.
 * e.g. "UTC-05:00"
 */
export function getTimezoneOffset(timezone: string): string | null {
    try {
        const formatter = new Intl.DateTimeFormat("en-US", {
            timeZone: timezone,
            timeZoneName: "shortOffset",
        })
        const parts = formatter.formatToParts(new Date())
        const offsetPart = parts.find((p) => p.type === "timeZoneName")
        return offsetPart?.value ?? null
    } catch {
        return null
    }
}
