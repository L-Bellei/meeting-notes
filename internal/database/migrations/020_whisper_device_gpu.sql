-- 020_whisper_device_gpu.sql
-- O seletor deixa de expor o backend: "cuda" vira "gpu" (o audio-service escolhe
-- CUDA ou Vulkan conforme a placa). auto/cpu ficam como estão. Reversível.
UPDATE settings SET value = 'gpu' WHERE key = 'whisper_device' AND value = 'cuda';
