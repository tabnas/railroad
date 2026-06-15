/* Copyright (c) 2026 Richard Rodger and other contributors, MIT License */

/*  renderer.ts
 *  Back-compat re-export surface. The implementation now lives in
 *  model.ts (diagram model + text), extract.ts (grammar introspection),
 *  svg.ts (vertical SVG), and ascii.ts (vertical ASCII). This module
 *  keeps the historical import paths working.
 */

export * from './model'
export { extractGrammar } from './extract'
export type { ExtractOptions } from './extract'
export { modelToSvg, renderNodeSvg, renderNodeSvg as toSvg } from './svg'
export type { SvgOptions } from './svg'
export { modelToAscii, renderNodeAscii } from './ascii'
export type { AsciiOptions } from './ascii'
