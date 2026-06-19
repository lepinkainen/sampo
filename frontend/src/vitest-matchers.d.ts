// Pulls the @testing-library/jest-dom matcher augmentation into the TS program
// so svelte-check recognizes matchers like toBeDisabled / toBeInTheDocument in
// *.test.ts files. The runtime registration lives in vitest-setup.ts.
import '@testing-library/jest-dom/vitest';
