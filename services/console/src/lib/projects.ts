import stc from 'string-to-color';
import Color from 'color';
import md5 from 'md5';

import icons from './icons.json';

function getDeterministicIndex(name: string, listLength: number): number {
    const hash = md5(name);
    let total = 0;

    // Turn full 128-bit hash into a number
    for (let i = 0; i < hash.length; i++) {
        total = (total * 31 + hash.charCodeAt(i)) >>> 0; // unsigned 32-bit
    }

    return total % listLength;
}

export function getRandomIcon(name: string): string {
    const idx = getDeterministicIndex(name, icons.length);
    return icons[idx];
}

export function getRandomColor(name: string): string {
    const color = new Color(stc(name)).darken(0.2).saturate(0.3);
    return color.hex();
}