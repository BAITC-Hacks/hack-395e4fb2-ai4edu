import '@testing-library/jest-dom/vitest';
import {afterEach, beforeEach, vi} from 'vitest';
import {cleanup} from '@testing-library/react';
beforeEach(()=>{localStorage.clear();vi.stubGlobal('scrollTo',vi.fn());});
afterEach(()=>{cleanup();vi.unstubAllGlobals();vi.useRealTimers();});
