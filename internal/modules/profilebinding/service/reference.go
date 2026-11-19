package service

// ReferenceCheck reports saved references that must survive Profile deletion.
// Ordinary metadata edits never invoke it. Bind before serving requests.
type ReferenceCheck func(profileID string) (bool, error)

var referenceCheck ReferenceCheck

func SetReferenceCheck(check ReferenceCheck) { referenceCheck = check }

func (s *Service) checkDeleteReferences(id string) error {
	if s.referenceCheck == nil {
		return nil
	}
	used, err := s.referenceCheck(id)
	if err != nil {
		return err
	}
	if used {
		return ErrProfileReferenced
	}
	return nil
}
