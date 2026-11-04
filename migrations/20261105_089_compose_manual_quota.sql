ALTER TABLE compose_strategies DROP CHECK chk_compose_strategies_limits,
 ADD CONSTRAINT chk_compose_strategies_limits CHECK(single_limit BETWEEN 1 AND 100 AND ((execution_mode='manual' AND daily_limit BETWEEN 0 AND 1000) OR (execution_mode='scheduled' AND daily_limit BETWEEN 1 AND 1000)));
