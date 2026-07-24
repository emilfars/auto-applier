import type { BuildOptions, BuildResult } from "esbuild";

export declare const entryPoints: string[];
export declare const baseOptions: BuildOptions;
export declare function runBuild(
  overrides?: BuildOptions,
): Promise<BuildResult>;
