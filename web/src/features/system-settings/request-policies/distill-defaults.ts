export type DistillRules = {
  hard_needles: string[]
  patterns: string[]
  fingerprints: string[]
  needles: string[]
  cot_patterns: string[]
  structure: {
    min_max_tokens: number
    require_no_tools: boolean
    require_single_turn: boolean
    contest_pattern: string
  }
}

export const factoryDistillRules: DistillRules = {
  hard_needles: [
    'Memory-stage-one extractor',
    'MUST return strict JSON only',
    'MUST distill reusable',
    'MUST extract durable memory',
    'durable rollout knowledge',
    'You MUST extract durable memory now',
  ],
  patterns: [],
  fingerprints: [
    'Let  $a,b,A,B$  be given reals. We consider the function defined by',
    String.raw`The graph of the function $y=\cos x - \sin x$ has a line of symmetry given by`,
    String.raw`Prove that if $\alpha, \beta, \gamma$ are the angles of a triangle, then`,
    "Jia walks from home to Yi's house. At the same time, Yi rides a bicycle",
    'Colonoscopy reveals a fungating hemorrhagic mass in the ascending colon',
    'reactant: Cc1ccc2c(cnn2C2CCCCO2)c1B1OC(C)(C)C(C)(C)O1',
    'What is the area, in square units, of an isosceles right triangle with a hypotenuse of 20 units?',
  ],
  needles: [
    '<answer>',
    '<|begin_of_thought|>',
    '<|end_of_thought|>',
    '<|begin_of_solution|>',
    String.raw`Return your final response within \boxed{}`,
    'Generate an executable Python function generated from the given prompt',
    'systematic long thinking process',
    'as an internal monologue',
    String.raw`Please reason step by step, and put your final answer within \boxed{}`,
  ],
  cot_patterns: [
    String.raw`\bdistill(?:ation|ing)?\s+(?:of\s+)?(?:(?:the|your|a|an|reusable|durable|rollout|hidden|internal|full|complete)\s+)?(?:reasoning(?:\s+traces?)?|chain[- ]of[- ]thoughts?|teacher(?:\s+model)?)\b`,
    String.raw`\b(?:extract|export|dump|reveal|harvest|exfiltrate)(?:ing|ed|ion|s)?\s+(?:(?:the|your|a|an|full|hidden|internal|complete|raw|durable|entire)\s+){0,4}(?:chain[- ]of[- ]thoughts?|reasoning\s+traces?|hidden\s+reasoning|internal\s+reasoning|internal\s+monologues?)\b`,
    String.raw`\b(?:chain[- ]of[- ]thought|reasoning\s+trace)\s+extraction\b`,
    String.raw`(?:思维链蒸馏|推理蒸馏|思考链蒸馏)`,
    String.raw`(?:提取|导出|抽取|蒸馏|收割|扒取)\s*(?:出|取)?\s*(?:你的|本人的)?\s*(?:完整|全部|隐藏|内部)?\s*(?:的)?\s*(?:思维链|思考链|推理链)`,
    String.raw`(?:思维链|思考链|推理链)\s*(?:的)?\s*(?:提取|导出|抽取|蒸馏|收割)`,
    String.raw`(?:提取|导出|抽取)\s*(?:出)?\s*(?:隐藏|内部)\s*(?:的)?\s*(?:推理|思维|思考)`,
  ],
  structure: {
    min_max_tokens: 8192,
    require_no_tools: true,
    require_single_turn: true,
    contest_pattern: String.raw`prove that|\\boxed|\baime\b|olympiad|\bgpqa\b|which of the following|livecodebench|amc[_\s-]?aime`,
  },
}

export const factoryDistillRulesJSON = JSON.stringify(factoryDistillRules)
