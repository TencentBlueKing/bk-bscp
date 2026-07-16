import { EnvType } from '../../types/env';

export const getEnvObj = (envDisplay: string): { envName: string; type: EnvType } => {
  if (!envDisplay) {
    return { envName: '--', type: EnvType.PRODUCTION };
  }
  const lastDashIndex = envDisplay.lastIndexOf('-');
  if (lastDashIndex === -1) {
    return { envName: envDisplay, type: EnvType.PRODUCTION };
  }
  return {
    envName: envDisplay.substring(0, lastDashIndex),
    type: envDisplay.substring(lastDashIndex + 1) as EnvType
  };
};
