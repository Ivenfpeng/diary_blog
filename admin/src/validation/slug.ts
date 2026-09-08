export const slugPatternSource = String.raw`[\p{L}\p{N}]+(?:-[\p{L}\p{N}]+)*`

export const slugRegExp = new RegExp(`^(?:${slugPatternSource})$`, 'u')

export const slugValidationMessage = 'Slug must use Chinese or other letters, numbers, and single hyphens.'
