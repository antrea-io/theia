.PHONY: all
all: policy_recommendation.zip drop_detection.zip

policy_recommendation.zip: policy_recommendation/*.py
	@zip $@ $^

drop_detection.zip: drop_detection/*.py
	@zip $@ $^

.PHONY: clean
clean:
	rm -f *.zip

.PHONY: check
check:
	PYTHONPATH="${PYTHONPATH}:$(CURDIR)" python3 -m unittest discover policy_recommendation "*_test.py"
	PYTHONPATH="${PYTHONPATH}:$(CURDIR)" python3 -m unittest discover drop_detection "*_test.py"
