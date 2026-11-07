// $state.snapshot pierde los tipos de las tuplas: esto los conserva.
export const foto = <T>(x: T): T => $state.snapshot(x) as T;
